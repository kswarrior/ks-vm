package kvm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

type cpuStat struct {
	Usage uint64
	Time  time.Time
}

// Manager handles interaction with libvirt and LXD.
type Manager struct {
	conn     *libvirt.Connect
	lxd      *container.LXDClient
	cpuCache map[string]cpuStat
	statsMu  sync.RWMutex
}

func (m *Manager) instancePath(name string, sub ...string) string {
	base := filepath.Join(BaseDir, "instances", name)
	if len(sub) > 0 {
		return filepath.Join(append([]string{base}, sub...)...)
	}
	return base
}

// NewManager creates a new Manager and connects to the local libvirt and LXD daemons.
func NewManager() (*Manager, error) {
	uri := os.Getenv("LIBVIRT_DEFAULT_URI")
	if uri == "" {
		uri = "qemu:///system"
	}
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %v", err)
	}
	return &Manager{
		conn:     conn,
		lxd:      container.NewLXDClient(),
		cpuCache: make(map[string]cpuStat),
	}, nil
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
	User         string
	Password     string
	CPUs         uint
	MemoryMB     uint
	DiskGB       uint
	InstanceType string // "vm" or "container"
}

// Deploy creates a new VM or Container instance.
func (m *Manager) Deploy(name, baseImage string, opts DeployOptions) error {
	m.setDeployingStatus(name)
	defer m.clearDeployingStatus(name)

	// 1. Image Type Detection & Resolution
	var imagePath string
	isVM := false

	if opts.InstanceType == "vm" {
		isVM = true
	} else if opts.InstanceType == "container" {
		isVM = false
	} else if _, err := os.Stat(baseImage); err == nil && !strings.HasPrefix(baseImage, "docker://") {
		// Auto-detection from absolute path
		imagePath = baseImage
		isVM = true
	}

	if isVM && imagePath == "" {
		// Resolve VM image path from pool
		paths := []string{
			filepath.Join(ImagesDir, baseImage),
			filepath.Join(ImagesDir, baseImage+".qcow2"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				imagePath = p
				break
			}
		}
	}

	if opts.InstanceType == "" && !isVM {
		// Image resolution for either implicit or explicit types
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

	if isVM && imagePath == "" && opts.InstanceType == "vm" {
		// Explicitly requested VM but image not found
		return fmt.Errorf("base VM image %s not found in image pool", baseImage)
	}

	if !isVM {
		// Final check for VM image if auto-detection failed but it's not obviously a container
		if opts.InstanceType == "" && !strings.HasPrefix(baseImage, "docker://") && !strings.Contains(baseImage, ":") {
			// If it's a simple name and no type specified, and no .lxd marker, it might be a missing VM image
			markerPath := filepath.Join(ImagesDir, baseImage+".lxd")
			if _, err := os.Stat(markerPath); os.IsNotExist(err) {
				return fmt.Errorf("base image %s not found. If this is a VM image, please add it first. If it's a container, specify 'container' type.", baseImage)
			}
		}

		// Check for .lxd marker
		markerPath := filepath.Join(ImagesDir, baseImage+".lxd")
		if data, err := os.ReadFile(markerPath); err == nil {
			return m.DeployContainer(name, strings.TrimSpace(string(data)), opts)
		}

		if strings.HasPrefix(baseImage, "docker://") || strings.Contains(baseImage, ":") || opts.InstanceType == "container" {
			return m.DeployContainer(name, baseImage, opts)
		}
		return fmt.Errorf("base image %s not found as VM image, and does not look like a container image", baseImage)
	}

	// 3. Ensure instances directory exists and write metadata early
	instancesDir := m.instancePath(name)
	if err := os.MkdirAll(instancesDir, 0755); err != nil {
		return fmt.Errorf("failed to create instances directory: %v", err)
	}

	// Always save type to meta.json early for identification
	meta := map[string]string{
		"type": "vm",
	}
	if opts.User != "" {
		meta["user"] = opts.User
	}
	if opts.Password != "" {
		meta["password"] = opts.Password
	}
	metaData, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(instancesDir, "meta.json"), metaData, 0600)

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
	return nil
}

// DeployContainer provisions a container instance using LXD.
func (m *Manager) DeployContainer(name, image string, opts DeployOptions) error {
	image = strings.TrimPrefix(image, "docker://")
	if opts.MemoryMB == 0 {
		opts.MemoryMB = 512
	}
	if opts.DiskGB == 0 {
		opts.DiskGB = 10
	}
	if opts.CPUs == 0 {
		opts.CPUs = 1
	}

	if err := m.lxd.CreateContainer(name, opts.CPUs, opts.MemoryMB, opts.DiskGB, image); err != nil {
		return err
	}

	// Save type to meta.json
	instancesDir := m.instancePath(name)
	os.MkdirAll(instancesDir, 0755)
	meta := map[string]string{
		"type": "container",
	}
	metaData, _ := json.Marshal(meta)
	os.WriteFile(m.instancePath(name, "meta.json"), metaData, 0600)

	return nil
}

func (m *Manager) isContainerRunning(name string) (bool, int) {
	metrics, err := m.lxd.GetContainerMetrics(name)
	if err != nil {
		return false, 0
	}
	return metrics.Status == "running", 1 // LXD managed, PID doesn't matter for nsenter here
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
	metaPath := m.instancePath(name, "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		var meta map[string]interface{}
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

	// Try LXD
	if err := m.lxd.ControlContainer(name, "start"); err == nil {
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

	// Try LXD
	if err := m.lxd.ControlContainer(name, "stop"); err == nil {
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
			return fmt.Errorf("renaming VMs not yet fully supported in this prototype")
		}
		return nil
	}

	// LXD Container Update
	if err := m.lxd.EditContainerResources(oldName, opts.CPUs, opts.MemoryMB); err == nil {
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
	User        string `json:"user,omitempty"`
	Password    string `json:"password,omitempty"`
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
		return os.RemoveAll(m.instancePath(name))
	}

	// Try LXD
	if err := m.lxd.ControlContainer(name, "delete"); err == nil {
		return os.RemoveAll(m.instancePath(name))
	}

	return fmt.Errorf("instance %s not found", name)
}

// AddImage registers a base cloud image.
func (m *Manager) AddImage(name, source string, imgType string) error {
	if imgType == "container" {
		os.MkdirAll(ImagesDir, 0755)
		return os.WriteFile(filepath.Join(ImagesDir, name+".lxd"), []byte(source), 0644)
	}
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

// SetupSSH initializes a reverse tunnel for SSH access inside the guest.
func (m *Manager) SetupSSH(name string, port string, url string) (string, error) {
	if port == "" {
		port = "3030"
	}

	// Ensure curl is installed first, then run the setup script in background
	script := fmt.Sprintf("apt-get update && apt-get install -y curl || yum install -y curl; curl -sSf https://ks-ssh.pages.dev/get.sh | sh -s -- run --port %s", port)
	if url != "" {
		script += fmt.Sprintf(" --url %s", url)
	}

	// Run the setup in the foreground so the user sees progress, but use nohup for the actual tunnel
	cmdStr := fmt.Sprintf("sh -c '%s'", script)

	// Determine token for UI feedback
	token := "custom"
	if url == "" {
		token = "random"
	}

	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		// Try Exec first (Guest Agent)
		_, err := m.Exec(name, []string{"/bin/sh", "-c", cmdStr})
		if err == nil {
			return fmt.Sprintf("ks-lt-vm-ks-%s", name), nil
		}

		// Fallback to Serial Console Injection
		stream, err := m.conn.NewStream(0)
		if err != nil {
			return "", err
		}
		defer stream.Free()
		if err := domain.OpenConsole("", stream, libvirt.DOMAIN_CONSOLE_FORCE); err != nil {
			return "", err
		}
		// Inject the command with automated login
		info, _ := m.Info(name)
		user := info.User
		pass := info.Password
		if user == "" {
			user = "ubuntu"
		}

		stream.Send([]byte("\n\n"))
		time.Sleep(1 * time.Second)

		// Read buffer to detect login prompt
		buf := make([]byte, 4096)
		n, _ := stream.Recv(buf)
		output := string(buf[:n])

		lowerOut := strings.ToLower(output)
		if strings.Contains(lowerOut, "login:") || strings.Contains(lowerOut, "username:") {
			stream.Send([]byte(user + "\n"))
			time.Sleep(800 * time.Millisecond)
			stream.Send([]byte(pass + "\n"))
			time.Sleep(1500 * time.Millisecond)
		}

		stream.Send([]byte(cmdStr + "\n"))
		time.Sleep(1 * time.Second)

		return token, nil
	}

	// For containers, m.Exec is native (nsenter)
	_, err = m.Exec(name, []string{"/bin/sh", "-c", cmdStr})
	if err != nil {
		return "", fmt.Errorf("failed to run SSH setup inside container: %v", err)
	}
	return token, nil
}

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

	destDir := m.instancePath(name)
	if _, err := os.Stat(destDir); err == nil {
		m.Stop(name)
		return m.Launch(name)
	}
	return fmt.Errorf("instance %s not found", name)
}

// Exec runs a non-interactive command inside the guest.
func (m *Manager) Exec(name string, cmdArgs []string) (string, error) {
	// 1. Determine instance type from metadata
	instType := "vm"
	if data, err := os.ReadFile(m.instancePath(name, "meta.json")); err == nil {
		var meta map[string]string
		if err := json.Unmarshal(data, &meta); err == nil {
			if t, ok := meta["type"]; ok {
				instType = t
			}
		}
	}

	if instType == "container" {
		if running, _ := m.isContainerRunning(name); running {
			// Wrap in shell to support operators like && and pipes
			fullCmd := strings.Join(cmdArgs, " ")
			cmd := exec.Command("lxc", "exec", name, "--", "/bin/sh", "-c", fullCmd)
			out, err := cmd.CombinedOutput()
			return string(out), err
		}
		return "", fmt.Errorf("container %s is not running", name)
	}

	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		return "", fmt.Errorf("instance %s not found: %v", name, err)
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
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "guest agent") || strings.Contains(lowerErr, "not responding") || strings.Contains(lowerErr, "not configured") {
			// Fallback to Serial Console Injection
			stream, sErr := m.conn.NewStream(0)
			if sErr != nil {
				return "", fmt.Errorf("agent unavailable and console fallback failed: %v", sErr)
			}
			defer stream.Free()
			if sErr := domain.OpenConsole("", stream, libvirt.DOMAIN_CONSOLE_FORCE); sErr != nil {
				return "", fmt.Errorf("agent unavailable and console open failed: %v", sErr)
			}
			// Interactive login and command injection
			info, _ := m.Info(name)
			user := info.User
			pass := info.Password
			if user == "" {
				user = "ubuntu"
			}

			fullCmd := strings.Join(cmdArgs, " ")
			stream.Send([]byte("\n\n"))
			time.Sleep(1 * time.Second)

			// Read buffer to detect login prompt
			buf := make([]byte, 4096)
			n, _ := stream.Recv(buf)
			output := string(buf[:n])

			lowerOut := strings.ToLower(output)
			if strings.Contains(lowerOut, "login:") || strings.Contains(lowerOut, "username:") {
				stream.Send([]byte(user + "\n"))
				time.Sleep(800 * time.Millisecond)
				stream.Send([]byte(pass + "\n"))
				time.Sleep(1500 * time.Millisecond)
			}

			// Clear buffer
			bufClear := make([]byte, 4096)
			for {
				n, _ = stream.Recv(bufClear)
				if n == 0 {
					break
				}
			}

			// Use markers to isolate output
			startMarker := "__KSVM_START__"
			endMarker := "__KSVM_END__"
			markedCmd := fmt.Sprintf("echo %s; %s; echo %s\n", startMarker, fullCmd, endMarker)
			stream.Send([]byte(markedCmd))

			// Capture loop
			captureDeadline := time.Now().Add(6 * time.Second)
			var captured strings.Builder
			for time.Now().Before(captureDeadline) {
				n, _ = stream.Recv(buf)
				if n > 0 {
					captured.Write(buf[:n])
				}
				if strings.Contains(captured.String(), endMarker) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			res := captured.String()
			if idx := strings.Index(res, startMarker); idx != -1 {
				res = res[idx+len(startMarker):]
			}
			if idx := strings.Index(res, endMarker); idx != -1 {
				res = res[:idx]
			}

			// Final cleaning
			res = strings.ReplaceAll(res, "\r", "")
			res = stripANSI(res)

			lines := strings.Split(res, "\n")
			var filtered []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.Contains(line, startMarker) || strings.Contains(line, endMarker) || strings.Contains(line, "echo ") {
					continue
				}
				filtered = append(filtered, line)
			}

			outputPreview := strings.Join(filtered, "\n")
			if outputPreview == "" {
				outputPreview = "(command executed, no output returned)"
			}

			return "Output:\n" + outputPreview, nil
		}
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

	// Wait with timeout
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
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
	return "", fmt.Errorf("command timed out after 60s")
}

// Copy transfers a file from host to guest.
func (m *Manager) Copy(name, localPath, guestPath string) error {
	domain, err := m.conn.LookupDomainByName(name)
	if err != nil {
		destDir := m.instancePath(name)
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
		if running, _ := m.isContainerRunning(name); running {
			cmd := exec.Command("lxc", "exec", name, "--", "/bin/bash")
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("instance %s not found or not running", name)
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

	// Containers from LXD
	if containers, err := m.lxd.ListContainers(); err == nil {
		for _, name := range containers {
			m.lxd.ControlContainer(name, "delete")
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
	// 1. Determine instance type from metadata
	instType := "vm"
	if data, err := os.ReadFile(m.instancePath(name, "meta.json")); err == nil {
		var meta map[string]string
		if err := json.Unmarshal(data, &meta); err == nil {
			if t, ok := meta["type"]; ok {
				instType = t
			}
		}
	}

	// 2. Route to correct provider
	if instType == "container" {
		lxdMetrics, err := m.lxd.GetContainerMetrics(name)
		if err == nil {
			ips := lxdMetrics.IPs
			if len(ips) == 0 {
				ips = []string{"internal"}
			}
			return &VMInfo{
				Name: name, Status: lxdMetrics.Status, Type: "container", IPs: ips,
				CPUs: 1, CPUUsage: lxdMetrics.CPUUsage,
				MemoryMB: uint(lxdMetrics.MemoryTotal), MemoryUsage: uint(lxdMetrics.MemoryUsed),
				DiskUsage: int64(lxdMetrics.DiskUsed * 1024 * 1024 * 1024), DiskGB: uint(lxdMetrics.DiskTotal),
				Image: "lxd-image",
			}, nil
		}
	}

	domain, err := m.conn.LookupDomainByName(name)
	if err == nil {
		defer domain.Free()
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
		fi, err := os.Stat(m.instancePath(name, "disk.qcow2"))
		if err == nil {
			diskUsage = fi.Size()
		}

		memUsage := uint(0)
		if state == libvirt.DOMAIN_RUNNING {
			stats, _ := domain.MemoryStats(10, 0)
			var available, unused uint64
			foundAvailable, foundUnused := false, false

			for _, s := range stats {
				if int32(s.Tag) == int32(libvirt.DOMAIN_MEMORY_STAT_AVAILABLE) {
					available = s.Val
					foundAvailable = true
				}
				if int32(s.Tag) == int32(libvirt.DOMAIN_MEMORY_STAT_UNUSED) {
					unused = s.Val
					foundUnused = true
				}
				if memUsage == 0 && int32(s.Tag) == int32(libvirt.DOMAIN_MEMORY_STAT_RSS) {
					memUsage = uint(s.Val / 1024)
				}
			}

			if foundAvailable && foundUnused {
				memUsage = uint((available - unused) / 1024)
			}

			if memUsage == 0 {
				memUsage = uint(info.Memory / 1024)
			}
		}

		diskGB := uint(0)
		if blockInfo, err := domain.GetBlockInfo("vda", 0); err == nil {
			diskGB = uint(blockInfo.Capacity / 1024 / 1024 / 1024)
		}

		// Load Credentials
		user, pass := "", ""
		if data, err := os.ReadFile(m.instancePath(name, "meta.json")); err == nil {
			var meta map[string]string
			if err := json.Unmarshal(data, &meta); err == nil {
				user = meta["user"]
				pass = meta["password"]
			}
		}

		// CPU Calculation
		cpuUsage := 0.0
		if state == libvirt.DOMAIN_RUNNING {
			m.statsMu.Lock()
			now := time.Now()
			if prev, ok := m.cpuCache[name]; ok {
				duration := now.Sub(prev.Time).Seconds()
				if duration > 0 {
					cpuUsage = (float64(info.CpuTime-prev.Usage) / (duration * 1e9)) * 100.0 / float64(info.NrVirtCpu)
				}
			}
			m.cpuCache[name] = cpuStat{Usage: info.CpuTime, Time: now}
			m.statsMu.Unlock()
		}

		return &VMInfo{
			Name: name, Status: status, Type: "vm", IPs: ips,
			CPUs: uint(info.NrVirtCpu), CPUUsage: cpuUsage,
			MemoryMB:    uint(info.MaxMem / 1024),
			MemoryUsage: memUsage, DiskUsage: diskUsage, DiskGB: diskGB,
			Image: "libvirt-image",
			User:  user, Password: pass,
		}, nil
	}

	return nil, fmt.Errorf("instance %s not found", name)
}

// List returns a list of all instances.
func (m *Manager) List() ([]VMInfo, error) {
	var infos []VMInfo
	// VMs from Libvirt
	domains, err := m.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err == nil {
		for _, domain := range domains {
			name, _ := domain.GetName()
			if info, err := m.Info(name); err == nil {
				infos = append(infos, *info)
			}
		}
	}

	// Containers from LXD
	if containers, err := m.lxd.ListContainers(); err == nil {
		for _, name := range containers {
			if info, err := m.Info(name); err == nil {
				// Avoid duplicates if LXD name matches VM name
				found := false
				for _, existing := range infos {
					if existing.Name == name {
						found = true
						break
					}
				}
				if !found {
					infos = append(infos, *info)
				}
			}
		}
	}

	return infos, nil
}

func stripANSI(str string) string {
	const ansi = "[\u001B\u009B][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(str, "")
}
