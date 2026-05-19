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
	"ksvm/pkg/container"
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

// Deploy creates a new VM or Container instance.
func (m *Manager) Deploy(name, baseImage string) error {
	// 1. Image Type Detection
	if strings.HasPrefix(baseImage, "docker://") || (!strings.HasSuffix(baseImage, ".qcow2") && !strings.Contains(baseImage, "/")) {
		return m.DeployContainer(name, baseImage)
	}

	// 2. Resolve base image path
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

	// 3. Ensure instances directory exists
	instancesDir := filepath.Join(BaseDir, "instances", name)
	if err := os.MkdirAll(instancesDir, 0755); err != nil {
		return fmt.Errorf("failed to create instances directory: %v", err)
	}

	diskPath := filepath.Join(instancesDir, "disk.qcow2")

	// 4. Create QCOW2 overlay
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", imagePath, "-F", "qcow2", diskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create overlay disk: %v, output: %s", err, string(output))
	}

	// 5. Generate XML
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

	// 6. Define and start the VM
	domain, err := m.conn.DomainDefineXML(xml)
	if err != nil {
		return fmt.Errorf("failed to define domain: %v", err)
	}

	if err := domain.Create(); err != nil {
		return fmt.Errorf("failed to start domain: %v", err)
	}

	return nil
}

// DeployContainer provisions a container instance.
func (m *Manager) DeployContainer(name, image string) error {
	destDir := filepath.Join(BaseDir, "containers", name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Unpack image
	if err := container.PullAndUnpack(image, destDir); err != nil {
		return err
	}

	// Record metadata
	meta := map[string]string{
		"type": "container",
		"image": image,
		"status": "stopped",
	}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(destDir, "meta.json"), metaJSON, 0644)

	return nil
}

// Launch starts a stopped VM or Container.
func (m *Manager) Launch(name string) error {
	// Try VM first
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		if err := domain.Create(); err != nil {
			return fmt.Errorf("failed to start domain %s: %v", name, err)
		}
		return nil
	}

	// Try Container
	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		metaData, _ := os.ReadFile(filepath.Join(destDir, "meta.json"))
		var meta map[string]string
		json.Unmarshal(metaData, &meta)

		go container.Run(name, destDir, []string{"/bin/sh"})

		meta["status"] = "running"
		metaJSON, _ := json.Marshal(meta)
		os.WriteFile(filepath.Join(destDir, "meta.json"), metaJSON, 0644)
		return nil
	}

	return fmt.Errorf("instance %s not found", name)
}

// Stop gracefully shuts down a VM or Container.
func (m *Manager) Stop(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		if err := domain.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown domain %s: %v", name, err)
		}
		return nil
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		if err := container.Stop(destDir); err != nil {
			return err
		}

		metaData, _ := os.ReadFile(filepath.Join(destDir, "meta.json"))
		var meta map[string]string
		json.Unmarshal(metaData, &meta)
		meta["status"] = "stopped"
		metaJSON, _ := json.Marshal(meta)
		os.WriteFile(filepath.Join(destDir, "meta.json"), metaJSON, 0644)
		return nil
	}

	return fmt.Errorf("instance %s not found", name)
}

// Suspend pauses a running VM.
func (m *Manager) Suspend(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}
	return domain.Suspend()
}

// Resume continues a suspended VM.
func (m *Manager) Resume(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}
	return domain.Resume()
}

// Update modifies VM resources (CPU/Memory).
func (m *Manager) Update(name string, memoryMB, cpus uint) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("failed to find domain %s: %v", name, err)
	}
	if err := domain.SetMemoryFlags(uint64(memoryMB)*1024, 0); err != nil {
		return fmt.Errorf("failed to set memory: %v", err)
	}
	if err := domain.SetVcpusFlags(cpus, 0); err != nil {
		return fmt.Errorf("failed to set vcpus: %v", err)
	}
	return nil
}

// VMInfo contains information about an instance (VM or Container).
type VMInfo struct {
	Name      string
	Status    string
	Type      string // "vm" or "container"
	IPs       []string
	CPUs      uint
	MemoryMB  uint
	DiskUsage int64
}

// Delete stops, destroys, and removes the VM/Container and its storage.
func (m *Manager) Delete(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		isActive, _ := domain.IsActive()
		if isActive {
			domain.Destroy()
		}
		domain.Undefine()
		os.RemoveAll(filepath.Join(BaseDir, "instances", name))
		return nil
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		container.Stop(destDir)
		return os.RemoveAll(destDir)
	}

	return fmt.Errorf("instance %s not found", name)
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

// Restart reboots a VM/Container gracefully.
func (m *Manager) Restart(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		if err := domain.Reboot(0); err != nil {
			isActive, _ := domain.IsActive()
			if isActive {
				return domain.Reset(0)
			}
			return err
		}
		return nil
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		m.Stop(name)
		return m.Launch(name)
	}

	return fmt.Errorf("instance %s not found", name)
}

// Exec runs a non-interactive command inside the guest.
func (m *Manager) Exec(name string, cmdArgs []string) (string, error) {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		destDir := filepath.Join(BaseDir, "containers", name)
		if _, err := os.Stat(destDir); err == nil {
			pidData, err := os.ReadFile(filepath.Join(destDir, "pid"))
			if err != nil { return "", fmt.Errorf("container not running") }
			cmd := exec.Command("nsenter", "-t", strings.TrimSpace(string(pidData)), "-m", "-u", "-i", "-n", "-p", "--")
			cmd.Args = append(cmd.Args, cmdArgs...)
			out, err := cmd.CombinedOutput()
			return string(out), err
		}
		return "", fmt.Errorf("instance %s not found", name)
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

// Copy transfers a file from host to guest.
func (m *Manager) Copy(name, localPath, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		destDir := filepath.Join(BaseDir, "containers", name)
		if _, err := os.Stat(destDir); err == nil {
			target := filepath.Join(destDir, "rootfs", guestPath)
			src, err := os.Open(localPath)
			if err != nil { return err }
			defer src.Close()
			dst, err := os.Create(target)
			if err != nil { return err }
			defer dst.Close()
			_, err = io.Copy(dst, src)
			return err
		}
		return fmt.Errorf("instance %s not found", name)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer file.Close()

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

// Umount detaches a shared directory.
func (m *Manager) Umount(name, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("umount not supported for containers")
	}

	unmountCmd := []string{"/usr/bin/umount", guestPath}
	m.Exec(name, unmountCmd)

	tag := "ksvm-mount-" + strings.ReplaceAll(guestPath, "/", "-")
	fs := libvirtxml.DomainFilesystem{
		Target: &libvirtxml.DomainFilesystemTarget{
			Dir: tag,
		},
	}
	xml, _ := fs.Marshal()
	return domain.DetachDeviceFlags(xml, 1)
}

// Mount dynamically attaches a host directory.
func (m *Manager) Mount(name, hostPath, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("mount not supported for containers")
	}

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
	xml, _ := fs.Marshal()
	if err := domain.AttachDeviceFlags(xml, 1); err != nil {
		return err
	}

	m.Exec(name, []string{"/usr/bin/mkdir", "-p", guestPath})
	m.Exec(name, []string{"/usr/bin/mount", "-t", "9p", "-o", "trans=virtio,version=9p2000.L", tag, guestPath})
	return nil
}

// Shell starts an interactive console session.
func (m *Manager) Shell(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		destDir := filepath.Join(BaseDir, "containers", name)
		if _, err := os.Stat(destDir); err == nil {
			pidData, err := os.ReadFile(filepath.Join(destDir, "pid"))
			if err != nil { return fmt.Errorf("container not running") }
			cmd := exec.Command("nsenter", "-t", strings.TrimSpace(string(pidData)), "-m", "-u", "-i", "-n", "-p", "/bin/sh")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
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

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil { return }
			_, err = stream.Send(buf[:n])
			if err != nil { return }
		}
	}()

	buf := make([]byte, 1024)
	for {
		n, err := stream.Recv(buf)
		if err != nil {
			if err == io.EOF { return nil }
			return err
		}
		os.Stdout.Write(buf[:n])
	}
}

// Purge destroys all VMs and wipes all storage.
func (m *Manager) Purge() error {
	domains, _ := m.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	for _, domain := range domains {
		isActive, _ := domain.IsActive()
		if isActive { domain.Destroy() }
		domain.Undefine()
	}
	return os.RemoveAll(BaseDir)
}

// VersionInfo contains version metadata.
type VersionInfo struct {
	KSVM    string
	Libvirt string
	QEMU    string
}

// Version returns the current versions.
func (m *Manager) Version() (*VersionInfo, error) {
	libVer, _ := m.conn.GetLibVersion()
	qemuVer, _ := m.conn.GetVersion()
	return &VersionInfo{
		KSVM:    "0.1.0-prototype",
		Libvirt: fmt.Sprintf("%d.%d.%d", libVer/1000000, (libVer%1000000)/1000, libVer%1000),
		QEMU:    fmt.Sprintf("%d.%d.%d", qemuVer/1000000, (qemuVer%1000000)/1000, qemuVer%1000),
	}, nil
}

// Info returns detailed information about an instance.
func (m *Manager) Info(name string) (*VMInfo, error) {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		info, _ := domain.GetInfo()
		state, _, _ := domain.GetState()
		status := "Unknown"
		switch state {
		case libvirt.DOMAIN_RUNNING: status = "Running"
		case libvirt.DOMAIN_PAUSED: status = "Paused"
		case libvirt.DOMAIN_SHUTOFF: status = "Stopped"
		}
		var ips []string
		ifaces, _ := domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
		for _, iface := range ifaces {
			for _, addr := range iface.Addrs { ips = append(ips, addr.Addr) }
		}
		return &VMInfo{
			Name: name, Status: status, Type: "vm", IPs: ips,
			CPUs: uint(info.NrVirtCpu), MemoryMB: uint(info.MaxMem / 1024),
		}, nil
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		metaData, _ := os.ReadFile(filepath.Join(destDir, "meta.json"))
		var meta map[string]string
		json.Unmarshal(metaData, &meta)
		return &VMInfo{
			Name: name, Status: meta["status"], Type: "container", IPs: []string{"internal"},
		}, nil
	}
	return nil, fmt.Errorf("instance %s not found", name)
}

// List returns a list of all instances.
func (m *Manager) List() ([]VMInfo, error) {
	var infos []VMInfo
	domains, _ := m.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	for _, domain := range domains {
		name, _ := domain.GetName()
		state, _, _ := domain.GetState()
		status := "Unknown"
		switch state {
		case libvirt.DOMAIN_RUNNING: status = "Running"
		case libvirt.DOMAIN_PAUSED: status = "Paused"
		case libvirt.DOMAIN_SHUTOFF: status = "Stopped"
		}
		infos = append(infos, VMInfo{Name: name, Status: status, Type: "vm"})
	}
	containerDir := filepath.Join(BaseDir, "containers")
	entries, _ := os.ReadDir(containerDir)
	for _, entry := range entries {
		if entry.IsDir() {
			metaData, _ := os.ReadFile(filepath.Join(containerDir, entry.Name(), "meta.json"))
			var meta map[string]string
			json.Unmarshal(metaData, &meta)
			infos = append(infos, VMInfo{Name: entry.Name(), Status: meta["status"], Type: "container"})
		}
	}
	return infos, nil
}
