package kvm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
)

const (
	BaseDir = "/var/lib/ksvm"
)

// Manager handles interaction with libvirt.
type Manager struct {
	conn *libvirt.Connect
}

// NewManager creates a new Manager and connects to the local libvirt daemon.
func NewManager() (*Manager, error) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %v", err)
	}
	return &Manager{conn: conn}, nil
}

// Close closes the libvirt connection.
func (m *Manager) Close() error {
	if m.conn == nil {
		return nil
	}
	_, err := m.conn.Close()
	return err
}

// Deploy creates a new VM from a base image using a QCOW2 overlay.
func (m *Manager) Deploy(name, baseImage string) error {
	// 1. Resolve base image path
	imagePath := filepath.Join(ImagesDir, baseImage)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		imagePath = filepath.Join(ImagesDir, baseImage+".qcow2")
		if _, err := os.Stat(imagePath); os.IsNotExist(err) {
			// Try as absolute path
			if _, err := os.Stat(baseImage); err != nil {
				return fmt.Errorf("base image %s not found", baseImage)
			}
			imagePath = baseImage
		}
	}

	// 2. Ensure instances directory exists
	instancesDir := filepath.Join(BaseDir, "instances", name)
	if err := os.MkdirAll(instancesDir, 0755); err != nil {
		return fmt.Errorf("failed to create instances directory: %v", err)
	}

	diskPath := filepath.Join(instancesDir, "disk.qcow2")

	// 3. Create QCOW2 overlay
	// qemu-img create -f qcow2 -b <imagePath> -F qcow2 <diskPath>
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", imagePath, "-F", "qcow2", diskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create overlay disk: %v, output: %s", err, string(output))
	}

	// 3. Generate XML
	config := VMConfig{
		Name:     name,
		MemoryMB: 1024,
		CPUs:     1,
		DiskPath: diskPath,
	}
	xml, err := GenerateDomainXML(config)
	if err != nil {
		return fmt.Errorf("failed to generate domain XML: %v", err)
	}

	// 4. Define and start the VM
	domain, err := m.conn.DomainDefineXML(xml)
	if err != nil {
		return fmt.Errorf("failed to define domain: %v", err)
	}

	if err := domain.Create(); err != nil {
		return fmt.Errorf("failed to start domain: %v", err)
	}

	return nil
}

// Launch starts a stopped VM.
func (m *Manager) Launch(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	if err := domain.Create(); err != nil {
		return fmt.Errorf("failed to start domain %s: %v", name, err)
	}

	return nil
}

// Stop gracefully shuts down a VM using ACPI.
func (m *Manager) Stop(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	if err := domain.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown domain %s: %v", name, err)
	}

	return nil
}

// VMInfo contains information about a VM.
type VMInfo struct {
	Name      string
	Status    string
	IPs       []string
	CPUs      uint
	MemoryMB  uint
	DiskUsage int64
}

// Delete stops, destroys, and removes the VM and its storage.
func (m *Manager) Delete(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	// Stop the domain if it's running
	isActive, err := domain.IsActive()
	if err != nil {
		return fmt.Errorf("failed to check if domain is active: %v", err)
	}

	if isActive {
		if err := domain.Destroy(); err != nil {
			return fmt.Errorf("failed to destroy domain: %v", err)
		}
	}

	// Undefine (remove from libvirt)
	if err := domain.Undefine(); err != nil {
		return fmt.Errorf("failed to undefine domain: %v", err)
	}

	// Remove storage
	instancesDir := filepath.Join(BaseDir, "instances", name)
	if err := os.RemoveAll(instancesDir); err != nil {
		return fmt.Errorf("failed to remove instance storage: %v", err)
	}

	return nil
}

// AddImage registers a base cloud image.
func (m *Manager) AddImage(name, source string) error {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		_, err := DownloadImage(name, source)
		return err
	}
	_, err := RegisterLocalImage(name, source)
	return err
}

// ListImages lists all registered base images.
func (m *Manager) ListImages() ([]ImageInfo, error) {
	return ListImages()
}

// RemoveImage removes a base image.
func (m *Manager) RemoveImage(name string) error {
	return RemoveImage(name)
}

// Restart reboots a VM gracefully, falling back to hard reset.
func (m *Manager) Restart(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	// Try graceful reboot first
	if err := domain.Reboot(0); err != nil {
		// Fallback to hard reset if domain is running
		isActive, _ := domain.IsActive()
		if isActive {
			return domain.Reset(0)
		}
		return err
	}

	return nil
}

// Exec runs a non-interactive command inside the guest via QEMU Guest Agent.
func (m *Manager) Exec(name string, cmdArgs []string) (string, error) {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return "", fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	execCmd := map[string]interface{}{
		"execute": "guest-exec",
		"arguments": map[string]interface{}{
			"path":           cmdArgs[0],
			"arg":            cmdArgs[1:],
			"capture-output": true,
		},
	}

	cmdJSON, _ := json.Marshal(execCmd)
	resp, err := domain.QemuAgentCommand(string(cmdJSON), -2, 0)
	if err != nil {
		return "", fmt.Errorf("qemu agent command failed: %v", err)
	}

	var startResult struct {
		Return struct {
			PID int `json:"pid"`
		} `json:"return"`
	}
	if err := json.Unmarshal([]byte(resp), &startResult); err != nil {
		return "", err
	}
	pid := startResult.Return.PID

	// Poll for completion
	for {
		statusCmd := map[string]interface{}{
			"execute": "guest-exec-status",
			"arguments": map[string]interface{}{
				"pid": pid,
			},
		}
		statusJSON, _ := json.Marshal(statusCmd)
		statusResp, err := domain.QemuAgentCommand(string(statusJSON), -2, 0)
		if err != nil {
			return "", err
		}

		var statusResult struct {
			Return struct {
				Exited   bool   `json:"exited"`
				ExitCode int    `json:"exitcode"`
				OutData  string `json:"out-data"`
				ErrData  string `json:"err-data"`
			} `json:"return"`
		}
		if err := json.Unmarshal([]byte(statusResp), &statusResult); err != nil {
			return "", err
		}

		if statusResult.Return.Exited {
			out, _ := base64.StdEncoding.DecodeString(statusResult.Return.OutData)
			errData, _ := base64.StdEncoding.DecodeString(statusResult.Return.ErrData)
			combined := string(out)
			if len(errData) > 0 {
				combined += "\nSTDERR:\n" + string(errData)
			}
			if statusResult.Return.ExitCode != 0 {
				return combined, fmt.Errorf("command failed with exit code %d", statusResult.Return.ExitCode)
			}
			return combined, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Copy transfers a file from host to guest via QEMU Guest Agent with chunking.
func (m *Manager) Copy(name, localPath, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer file.Close()

	// 1. Open file in guest
	openCmd := map[string]interface{}{
		"execute": "guest-file-open",
		"arguments": map[string]interface{}{
			"path": guestPath,
			"mode": "wb",
		},
	}
	openJSON, _ := json.Marshal(openCmd)
	respOpen, err := domain.QemuAgentCommand(string(openJSON), -2, 0)
	if err != nil {
		return fmt.Errorf("guest-file-open failed: %v", err)
	}

	var openResult struct {
		Return int `json:"return"`
	}
	if err := json.Unmarshal([]byte(respOpen), &openResult); err != nil {
		return fmt.Errorf("failed to parse guest-file-open response: %v", err)
	}
	handle := openResult.Return

	// 2. Write content in chunks (e.g., 32KB)
	buf := make([]byte, 32*1024)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			writeCmd := map[string]interface{}{
				"execute": "guest-file-write",
				"arguments": map[string]interface{}{
					"handle":   handle,
					"buf-b64": encoded,
				},
			}
			writeJSON, _ := json.Marshal(writeCmd)
			_, err = domain.QemuAgentCommand(string(writeJSON), -2, 0)
			if err != nil {
				return fmt.Errorf("guest-file-write failed: %v", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// 3. Close file
	closeCmd := map[string]interface{}{
		"execute": "guest-file-close",
		"arguments": map[string]interface{}{
			"handle": handle,
		},
	}
	closeJSON, _ := json.Marshal(closeCmd)
	_, err = domain.QemuAgentCommand(string(closeJSON), -2, 0)
	if err != nil {
		return fmt.Errorf("guest-file-close failed: %v", err)
	}

	return nil
}

// Umount detaches a shared directory from the VM.
func (m *Manager) Umount(name, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	// 1. Gracefully unmount in guest
	unmountCmd := []string{"/usr/bin/umount", guestPath}
	m.Exec(name, unmountCmd) // Ignore error if already unmounted

	// 2. Resolve the tag used during mount
	tag := "ksvm-mount-" + strings.ReplaceAll(guestPath, "/", "-")

	fs := libvirtxml.DomainFilesystem{
		Target: &libvirtxml.DomainFilesystemTarget{
			Dir: tag,
		},
	}

	xml, err := fs.Marshal()
	if err != nil {
		return err
	}

	// VIR_DOMAIN_DEVICE_MODIFY_LIVE = 1
	return domain.DetachDeviceFlags(xml, 1)
}

// Mount dynamically attaches a host directory to the VM and mounts it in the guest.
func (m *Manager) Mount(name, hostPath, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	// Use a unique tag for the mount
	tag := "ksvm-mount-" + strings.ReplaceAll(guestPath, "/", "-")

	fs := libvirtxml.DomainFilesystem{
		AccessMode: "passthrough",
		Source: &libvirtxml.DomainFilesystemSource{
			Mount: &libvirtxml.DomainFilesystemSourceMount{
				Dir: hostPath,
			},
		},
		Target: &libvirtxml.DomainFilesystemTarget{
			Dir: tag,
		},
	}

	xml, err := fs.Marshal()
	if err != nil {
		return err
	}

	// Hot-plug the device
	// VIR_DOMAIN_DEVICE_MODIFY_LIVE = 1
	if err := domain.AttachDeviceFlags(xml, 1); err != nil {
		return fmt.Errorf("failed to attach device: %v", err)
	}

	// Run mount command in guest
	mountCmd := []string{"/usr/bin/mkdir", "-p", guestPath}
	if _, err := m.Exec(name, mountCmd); err != nil {
		return fmt.Errorf("failed to create mount point in guest: %v", err)
	}

	mountCmd = []string{"/usr/bin/mount", "-t", "9p", "-o", "trans=virtio,version=9p2000.L", tag, guestPath}
	if _, err := m.Exec(name, mountCmd); err != nil {
		return fmt.Errorf("failed to mount in guest: %v", err)
	}

	return nil
}

// Shell starts an interactive console session.
func (m *Manager) Shell(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	stream, err := m.conn.NewStream(0)
	if err != nil {
		return err
	}
	defer stream.Free()

	if err := domain.OpenConsole("", stream, libvirt.DOMAIN_CONSOLE_FORCE); err != nil {
		return err
	}

	// Set stdin to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			_, err = stream.Send(buf[:n])
			if err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 1024)
	for {
		n, err := stream.Recv(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		os.Stdout.Write(buf[:n])
	}
}

// VersionInfo contains version metadata.
type VersionInfo struct {
	KSVM    string
	Libvirt string
	QEMU    string
}

// Purge destroys all VMs and wipes all storage.
func (m *Manager) Purge() error {
	domains, err := m.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err != nil {
		return err
	}

	for _, domain := range domains {
		isActive, _ := domain.IsActive()
		if isActive {
			domain.Destroy()
		}
		domain.Undefine()
	}

	return os.RemoveAll(BaseDir)
}

// Version returns the current versions of ksvm, libvirt, and QEMU.
func (m *Manager) Version() (*VersionInfo, error) {
	libVer, err := m.conn.GetLibVersion()
	if err != nil {
		return nil, err
	}
	qemuVer, err := m.conn.GetVersion()
	if err != nil {
		return nil, err
	}

	return &VersionInfo{
		KSVM:    "0.1.0-prototype",
		Libvirt: fmt.Sprintf("%d.%d.%d", libVer/1000000, (libVer%1000000)/1000, libVer%1000),
		QEMU:    fmt.Sprintf("%d.%d.%d", qemuVer/1000000, (qemuVer%1000000)/1000, qemuVer%1000),
	}, nil
}

// Info returns detailed information about a VM.
func (m *Manager) Info(name string) (*VMInfo, error) {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to find domain %s: %v", name, err)
	}

	info, err := domain.GetInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get domain info: %v", err)
	}

	state, _, _ := domain.GetState()
	status := "Unknown"
	switch state {
	case libvirt.DOMAIN_RUNNING:
		status = "Running"
	case libvirt.DOMAIN_PAUSED:
		status = "Paused"
	case libvirt.DOMAIN_SHUTOFF:
		status = "Stopped"
	}

	var ips []string
	if state == libvirt.DOMAIN_RUNNING {
		ifaces, err := domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
		if err == nil {
			for _, iface := range ifaces {
				for _, addr := range iface.Addrs {
					ips = append(ips, addr.Addr)
				}
			}
		}
	}

	// Disk usage - simple approach: check the instance disk size
	var diskUsage int64
	diskPath := filepath.Join(BaseDir, "instances", name, "disk.qcow2")
	if fi, err := os.Stat(diskPath); err == nil {
		diskUsage = fi.Size()
	}

	return &VMInfo{
		Name:      name,
		Status:    status,
		IPs:       ips,
		CPUs:      uint(info.NrVirtCpu),
		MemoryMB:  uint(info.MaxMem / 1024),
		DiskUsage: diskUsage,
	}, nil
}

// List returns a list of all VMs and their statuses.
func (m *Manager) List() ([]VMInfo, error) {
	domains, err := m.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %v", err)
	}

	var infos []VMInfo
	for _, domain := range domains {
		name, _ := domain.GetName()
		state, _, _ := domain.GetState()

		status := "Unknown"
		switch state {
		case libvirt.DOMAIN_RUNNING:
			status = "Running"
		case libvirt.DOMAIN_PAUSED:
			status = "Paused"
		case libvirt.DOMAIN_SHUTOFF:
			status = "Stopped"
		}

		var ips []string
		if state == libvirt.DOMAIN_RUNNING {
			ifaces, err := domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
			if err == nil {
				for _, iface := range ifaces {
					for _, addr := range iface.Addrs {
						ips = append(ips, addr.Addr)
					}
				}
			}
		}

		infos = append(infos, VMInfo{
			Name:   name,
			Status: status,
			IPs:    ips,
		})
	}

	return infos, nil
}
