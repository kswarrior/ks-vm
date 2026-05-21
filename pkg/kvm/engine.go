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
	"syscall"
	"time"

	"golang.org/x/term"
	"ksvm/pkg/container"
	"ksvm/pkg/web"
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
	uri := os.Getenv("LIBVIRT_DEFAULT_URI")
	if uri == "" {
		uri = "qemu:///system"
	}
	conn, err := libvirt.NewConnect(uri)
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

// DeployOptions contains optional parameters for deployment.
type DeployOptions struct {
	User     string
	Password string
	CPUs     uint
	MemoryMB uint
	DiskGB   uint
}

// Deploy creates a new VM or Container instance.
func (m *Manager) Deploy(name, baseImage string, opts DeployOptions) error {
	m.setDeployingStatus(name)
	// 1. Image Type Detection & Resolution
	var imagePath string
	isVM := false

	// Check if baseImage is an absolute or relative path to a file
	if _, err := os.Stat(baseImage); err == nil && !strings.HasPrefix(baseImage, "docker://") {
		imagePath = baseImage
		isVM = true
	} else {
		// Check in ImagesDir
		paths := []string{
			filepath.Join(ImagesDir, baseImage),
			filepath.Join(ImagesDir, baseImage+".qcow2"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				imagePath = p
				isVM = true
				break
			}
		}
	}

	if !isVM && (strings.HasPrefix(baseImage, "docker://") || !strings.Contains(baseImage, "/")) {
		return m.DeployContainer(name, baseImage)
	}

	if !isVM {
		return fmt.Errorf("base image %s not found as VM image, and does not look like a container image", baseImage)
	}

	// 3. Ensure instances directory exists
	instancesDir := filepath.Join(BaseDir, "instances", name)
	if err := os.MkdirAll(instancesDir, 0755); err != nil {
		return fmt.Errorf("failed to create instances directory: %v", err)
	}

	// Cleanup on failure
	deployed := false
	defer func() {
		if !deployed {
			os.RemoveAll(instancesDir)
		}
	}()

	diskPath := filepath.Join(instancesDir, "disk.qcow2")

	// 4. Create QCOW2 overlay
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", imagePath, "-F", "qcow2", diskPath)
	if opts.DiskGB > 0 {
		cmd.Args = append(cmd.Args, fmt.Sprintf("%dG", opts.DiskGB))
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create overlay disk: %v, output: %s", err, string(output))
	}

	// 5. Generate Cloud-Init ISO if credentials provided
	configDrivePath := ""
	if opts.User != "" && opts.Password != "" {
		iso, err := m.GenerateCloudInitISO(instancesDir, opts.User, opts.Password)
		if err != nil {
			return err
		}
		configDrivePath = iso
	}

	// 6. Generate XML
	mem := opts.MemoryMB
	if mem == 0 {
		mem = 1024
	}
	cpus := opts.CPUs
	if cpus == 0 {
		cpus = 1
	}

	config := VMConfig{
		Name:            name,
		MemoryMB:        mem,
		CPUs:            cpus,
		DiskPath:        diskPath,
		ConfigDrivePath: configDrivePath,
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
		domain.Undefine()
		return fmt.Errorf("failed to start domain: %v", err)
	}

	deployed = true
	m.clearDeployingStatus(name)

	return nil
}

// DeployContainer provisions a container instance.
func (m *Manager) DeployContainer(name, image string) error {
	destDir := filepath.Join(BaseDir, "containers", name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	deployed := false
	defer func() {
		if !deployed {
			os.RemoveAll(destDir)
		}
	}()

	if err := container.PullAndUnpack(image, destDir); err != nil {
		return err
	}

	meta := map[string]string{
		"type":   "container",
		"image":  image,
		"status": "stopped",
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destDir, "meta.json"), metaJSON, 0644); err != nil {
		return err
	}
	deployed = true
	m.clearDeployingStatus(name)
	return nil
}

func (m *Manager) isContainerRunning(name string) (bool, int) {
	destDir := filepath.Join(BaseDir, "containers", name)
	pidPath := filepath.Join(destDir, "pid")
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return false, 0
	}
	var pid int
	fmt.Sscanf(string(pidData), "%d", &pid)
	if pid <= 0 {
		return false, 0
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	// On Unix, FindProcess always succeeds. Use signal 0 to check existence.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Update meta.json if we detected it died
		m.updateContainerStatus(name, "stopped")
		return false, 0
	}
	return true, pid
}

func (m *Manager) setDeployingStatus(name string) {
	dir := filepath.Join(BaseDir, "deploying")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, name), []byte(time.Now().Format(time.RFC3339)), 0644)
}

func (m *Manager) clearDeployingStatus(name string) {
	os.Remove(filepath.Join(BaseDir, "deploying", name))
}

func (m *Manager) isDeploying(name string) bool {
	_, err := os.Stat(filepath.Join(BaseDir, "deploying", name))
	return err == nil
}

func (m *Manager) updateContainerStatus(name, status string) {
	destDir := filepath.Join(BaseDir, "containers", name)
	metaPath := filepath.Join(destDir, "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		var meta map[string]string
		if err := json.Unmarshal(metaData, &meta); err == nil {
			meta["status"] = status
			newMeta, _ := json.Marshal(meta)
			os.WriteFile(metaPath, newMeta, 0644)
		}
	}
}

// Launch starts a stopped VM or Container.
func (m *Manager) Launch(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		isActive, err := domain.IsActive()
		if err != nil {
			return err
		}
		if isActive {
			return fmt.Errorf("domain %s is already running", name)
		}
		return domain.Create()
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		alreadyRunning, _ := m.isContainerRunning(name)
		if alreadyRunning {
			return fmt.Errorf("container %s is already running", name)
		}

		// Set environment variable to run container in background
		os.Setenv("KSVM_BG", "1")
		defer os.Unsetenv("KSVM_BG")

		// Run setup in a long-running process to keep the container alive
		// We use a shell loop as a more universal way to keep the namespace open
		keepAliveCmd := []string{"/bin/sh", "-c", "while true; do sleep 3600; done"}
		if err := container.Run(name, destDir, keepAliveCmd); err != nil {
			return fmt.Errorf("failed to start container: %v", err)
		}

		// Verify liveness with retries
		var isRunning bool
		for i := 0; i < 5; i++ {
			time.Sleep(200 * time.Millisecond)
			if r, _ := m.isContainerRunning(name); r {
				isRunning = true
				break
			}
		}

		if !isRunning {
			logData, _ := os.ReadFile(filepath.Join(destDir, "container.log"))
			return fmt.Errorf("container failed to stay alive. Check /var/lib/ksvm/containers/%s/container.log. Last log: %s", name, string(logData))
		}

		metaData, err := os.ReadFile(filepath.Join(destDir, "meta.json"))
		if err == nil {
			var meta map[string]string
			if err := json.Unmarshal(metaData, &meta); err == nil {
				meta["status"] = "running"
				metaJSON, _ := json.Marshal(meta)
				os.WriteFile(filepath.Join(destDir, "meta.json"), metaJSON, 0644)
			}
		}
		return nil
	}
	return fmt.Errorf("instance %s not found", name)
}

// Stop gracefully shuts down a VM or Container.
func (m *Manager) Stop(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		isActive, err := domain.IsActive()
		if err != nil {
			return err
		}
		if !isActive {
			return fmt.Errorf("domain %s is not running", name)
		}
		return domain.Shutdown()
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		running, _ := m.isContainerRunning(name)
		if !running {
			return fmt.Errorf("container %s is not running", name)
		}

		if err := container.Stop(destDir); err != nil {
			return err
		}

		metaData, err := os.ReadFile(filepath.Join(destDir, "meta.json"))
		if err == nil {
			var meta map[string]string
			if err := json.Unmarshal(metaData, &meta); err == nil {
				meta["status"] = "stopped"
				metaJSON, _ := json.Marshal(meta)
				os.WriteFile(filepath.Join(destDir, "meta.json"), metaJSON, 0644)
			}
		}
		return nil
	}
	return fmt.Errorf("instance %s not found", name)
}

// Suspend pauses a running VM.
func (m *Manager) Suspend(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("suspend not supported for containers")
	}
	return domain.Suspend()
}

// Resume continues a suspended VM.
func (m *Manager) Resume(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("resume not supported for containers")
	}
	return domain.Resume()
}

// UpdateInstance modifies instance resources and metadata.
func (m *Manager) UpdateInstance(oldName, newName string, opts DeployOptions) error {
	domain, err := m.conn.LookupDomainByName(oldName)
	if err == nil {
		// VM Update
		if opts.MemoryMB > 0 {
			domain.SetMemoryFlags(uint64(opts.MemoryMB)*1024, libvirt.DOMAIN_MEM_CONFIG|libvirt.DOMAIN_MEM_LIVE)
		}
		if opts.CPUs > 0 {
			domain.SetVcpusFlags(opts.CPUs, libvirt.DOMAIN_VCPU_CONFIG|libvirt.DOMAIN_VCPU_LIVE)
		}

		if newName != "" && newName != oldName {
			isActive, _ := domain.IsActive()
			if isActive {
				return fmt.Errorf("cannot rename a running VM")
			}
			// Renaming in libvirt is tricky (undefine/define), for now we just change metadata if possible
			// or return error if not implemented fully.
			return fmt.Errorf("renaming VMs not yet fully supported in this prototype")
		}
		return nil
	}

	destDir := filepath.Join(BaseDir, "containers", oldName)
	if _, err := os.Stat(destDir); err == nil {
		// Container Update
		if newName != "" && newName != oldName {
			newDir := filepath.Join(BaseDir, "containers", newName)
			if err := os.Rename(destDir, newDir); err != nil {
				return err
			}
			destDir = newDir
		}

		metaData, err := os.ReadFile(filepath.Join(destDir, "meta.json"))
		if err == nil {
			var meta map[string]string
			json.Unmarshal(metaData, &meta)
			if opts.User != "" {
				meta["user"] = opts.User
			}
			newMeta, _ := json.Marshal(meta)
			os.WriteFile(filepath.Join(destDir, "meta.json"), newMeta, 0644)
		}
		return nil
	}

	return fmt.Errorf("instance %s not found", oldName)
}

// VMInfo contains information about an instance.
type VMInfo struct {
	Name        string
	Status      string
	Type        string
	IPs         []string
	CPUs        uint
	CPUUsage    float64
	MemoryMB    uint
	MemoryUsage uint
	DiskGB      uint
	DiskUsage   int64
	Image       string
}

// Delete stops, destroys, and removes the VM/Container and its storage.
func (m *Manager) Delete(name string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		isActive, err := domain.IsActive()
		if err == nil && isActive {
			domain.Destroy()
		}
		if err := domain.Undefine(); err != nil {
			return err
		}
		return os.RemoveAll(filepath.Join(BaseDir, "instances", name))
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
			if err != nil {
				return "", fmt.Errorf("container not running")
			}

			if len(cmdArgs) == 0 {
				return "", fmt.Errorf("no command provided")
			}
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
		return "", err
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
			"execute":   "guest-exec-status",
			"arguments": map[string]interface{}{"pid": pid},
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
				return combined, fmt.Errorf("code %d", statusResult.Return.ExitCode)
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
			if err != nil {
				return err
			}
			defer src.Close()
			dst, err := os.Create(target)
			if err != nil {
				return err
			}
			defer dst.Close()
			_, err = io.Copy(dst, src)
			return err
		}
		return fmt.Errorf("instance %s not found", name)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	openCmd := map[string]interface{}{
		"execute":   "guest-file-open",
		"arguments": map[string]interface{}{"path": guestPath, "mode": "wb"},
	}
	openJSON, _ := json.Marshal(openCmd)
	respOpen, err := domain.QemuAgentCommand(string(openJSON), -2, 0)
	if err != nil {
		return err
	}

	var openResult struct {
		Return int `json:"return"`
	}
	if err := json.Unmarshal([]byte(respOpen), &openResult); err != nil {
		return err
	}
	handle := openResult.Return

	buf := make([]byte, 32*1024)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			writeCmd := map[string]interface{}{
				"execute":   "guest-file-write",
				"arguments": map[string]interface{}{"handle": handle, "buf-b64": encoded},
			}
			writeJSON, _ := json.Marshal(writeCmd)
			domain.QemuAgentCommand(string(writeJSON), -2, 0)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	closeCmd := map[string]interface{}{
		"execute":   "guest-file-close",
		"arguments": map[string]interface{}{"handle": handle},
	}
	closeJSON, _ := json.Marshal(closeCmd)
	domain.QemuAgentCommand(string(closeJSON), -2, 0)
	return nil
}

// Umount detaches a shared directory.
func (m *Manager) Umount(name, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("not supported for containers")
	}
	m.Exec(name, []string{"/usr/bin/umount", guestPath})
	tag := "ksvm-mount-" + strings.ReplaceAll(guestPath, "/", "-")
	fs := libvirtxml.DomainFilesystem{Target: &libvirtxml.DomainFilesystemTarget{Dir: tag}}
	xml, _ := fs.Marshal()
	return domain.DetachDeviceFlags(xml, 1)
}

// Mount dynamically attaches a host directory.
func (m *Manager) Mount(name, hostPath, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("not supported for containers")
	}
	tag := "ksvm-mount-" + strings.ReplaceAll(guestPath, "/", "-")
	fs := libvirtxml.DomainFilesystem{
		AccessMode: "passthrough",
		Source:     &libvirtxml.DomainFilesystemSource{Mount: &libvirtxml.DomainFilesystemSourceMount{Dir: hostPath}},
		Target:     &libvirtxml.DomainFilesystemTarget{Dir: tag},
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
		running, pid := m.isContainerRunning(name)
		if running {
			// All namespaces: m=mount, u=uts, i=ipc, n=net, p=pid
			cmd := exec.Command("nsenter", "-t", fmt.Sprintf("%d", pid), "-m", "-u", "-i", "-n", "-p", "/bin/sh")
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		}

		destDir := filepath.Join(BaseDir, "containers", name)
		if _, err := os.Stat(destDir); err == nil {
			return fmt.Errorf("container %s is not running", name)
		}
		return fmt.Errorf("instance %s not found", name)
	}

	isActive, err := domain.IsActive()
	if err != nil {
		return fmt.Errorf("failed to check if domain is active: %v", err)
	}
	if !isActive {
		return fmt.Errorf("domain %s is not running", name)
	}

	stream, err := m.conn.NewStream(0)
	if err != nil {
		return fmt.Errorf("failed to create stream: %v", err)
	}
	defer stream.Free()

	// OpenConsole is the way to attach to the serial console
	if err := domain.OpenConsole("", stream, libvirt.DOMAIN_CONSOLE_FORCE); err != nil {
		return fmt.Errorf("failed to open console: %v", err)
	}

	// Make the terminal raw for interactive use
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("\r")
	web.PrintShellBanner(name)

	// Trigger a prompt by sending a newline after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		stream.Send([]byte("\n"))
	}()

	errChan := make(chan error, 2)

	// Host to VM
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if _, sErr := stream.Send(buf[:n]); sErr != nil {
					errChan <- sErr
					return
				}
			}
			if err == io.EOF {
				errChan <- nil
				return
			}
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	// VM to Host
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stream.Recv(buf)
			if n > 0 {
				if _, wErr := os.Stdout.Write(buf[:n]); wErr != nil {
					errChan <- wErr
					return
				}
			}
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	return <-errChan
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
	if err == nil {
		for _, domain := range domains {
			isActive, _ := domain.IsActive()
			if isActive {
				domain.Destroy()
			}
			domain.Undefine()
		}
	}
	return os.RemoveAll(BaseDir)
}

// Version returns the current versions.
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

// Info returns detailed information about an instance.
func (m *Manager) Info(name string) (*VMInfo, error) {
	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		info, err := domain.GetInfo()
		if err != nil {
			return nil, err
		}
		state, _, err := domain.GetState()
		if err != nil {
			return nil, err
		}
		status := "Unknown"
		switch state {
		case libvirt.DOMAIN_RUNNING:
			status = "running"
		case libvirt.DOMAIN_PAUSED:
			status = "paused"
		case libvirt.DOMAIN_SHUTOFF:
			status = "stopped"
		}
		if m.isDeploying(name) {
			status = "deploying"
		}

		var ips []string
		ifaces, _ := domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
		for _, iface := range ifaces {
			for _, addr := range iface.Addrs {
				ips = append(ips, addr.Addr)
			}
		}
		var diskUsage int64
		fi, err := os.Stat(filepath.Join(BaseDir, "instances", name, "disk.qcow2"))
		if err == nil {
			diskUsage = fi.Size()
		}

		memUsage := uint(0)
		if state == libvirt.DOMAIN_RUNNING {
			stats, _ := domain.MemoryStats(10, 0)
			for _, s := range stats {
				if int32(s.Tag) == int32(libvirt.DOMAIN_MEMORY_STAT_RSS) {
					memUsage = uint(s.Val / 1024)
				}
			}
		}

		return &VMInfo{
			Name: name, Status: status, Type: "vm", IPs: ips,
			CPUs: uint(info.NrVirtCpu), CPUUsage: 0.0, // Hard to calculate live CPU without interval
			MemoryMB:    uint(info.MaxMem / 1024),
			MemoryUsage: memUsage, DiskUsage: diskUsage, DiskGB: 0,
			Image: "libvirt-image",
		}, nil
	}

	destDir := filepath.Join(BaseDir, "containers", name)
	if _, err := os.Stat(destDir); err == nil {
		metaData, err := os.ReadFile(filepath.Join(destDir, "meta.json"))
		if err != nil {
			return nil, err
		}
		var meta map[string]string
		if err := json.Unmarshal(metaData, &meta); err != nil {
			return nil, err
		}
		var rootfsUsage int64
		filepath.Walk(filepath.Join(destDir, "rootfs"), func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				rootfsUsage += info.Size()
			}
			return nil
		})
		return &VMInfo{
			Name: name, Status: meta["status"], Type: "container", IPs: []string{"internal"},
			CPUs: 1, CPUUsage: 0.0, MemoryMB: 512, MemoryUsage: 0,
			DiskUsage: rootfsUsage, DiskGB: 1,
			Image: meta["image"],
		}, nil
	}
	return nil, fmt.Errorf("instance %s not found", name)
}

// List returns a list of all instances.
func (m *Manager) List() ([]VMInfo, error) {
	var infos []VMInfo
	domains, err := m.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err == nil {
		for _, domain := range domains {
			name, _ := domain.GetName()
			state, _, _ := domain.GetState()
			status := "Unknown"
			if m.isDeploying(name) {
				status = "deploying"
			} else {
				switch state {
				case libvirt.DOMAIN_RUNNING:
					status = "running"
				case libvirt.DOMAIN_PAUSED:
					status = "paused"
				case libvirt.DOMAIN_SHUTOFF:
					status = "stopped"
				}
			}
			var ips []string
			if status == "running" {
				ifaces, _ := domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
				for _, iface := range ifaces {
					for _, addr := range iface.Addrs {
						ips = append(ips, addr.Addr)
					}
				}
			}
			infos = append(infos, VMInfo{Name: name, Status: status, Type: "vm", IPs: ips})
		}
	}
	entries, _ := os.ReadDir(filepath.Join(BaseDir, "containers"))
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			status := "stopped"
			if m.isDeploying(name) {
				status = "deploying"
			} else {
				running, _ := m.isContainerRunning(name)
				if running {
					status = "running"
				}
			}
			infos = append(infos, VMInfo{Name: name, Status: status, Type: "container"})
		}
	}
	return infos, nil
}
