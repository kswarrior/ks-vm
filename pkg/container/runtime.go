package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Run starts a container using namespaces and pivot_root.
func Run(name, rootDir string, args []string) error {
	rootfs := filepath.Join(rootDir, "rootfs")

	// We re-execute the current process to set up namespaces
	if os.Getenv("KSVM_IN_CONTAINER") == "" {
		cmd := exec.Command("/proc/self/exe", append([]string{"internal-run", name, rootDir}, args...)...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNET,
			Setsid:     true,
		}
		cmd.Env = append(os.Environ(), "KSVM_IN_CONTAINER=1")

		if os.Getenv("KSVM_BG") == "1" {
			logPath := filepath.Join(rootDir, "container.log")
			logFile, err := os.Create(logPath)
			if err != nil {
				return err
			}
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			cmd.Stdin = nil
		} else {
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Start(); err != nil {
			return err
		}

		pidPath := filepath.Join(rootDir, "pid")
		os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)

		if os.Getenv("KSVM_BG") == "1" {
			return nil
		}
		return cmd.Wait()
	}

	// Inside the namespace
	if err := setupContainer(name, rootfs); err != nil {
		return err
	}

	// Set a basic PATH
	env := os.Environ()
	hasPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		env = append(env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}

	path, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("command %s not found: %v", args[0], err)
	}

	return syscall.Exec(path, args, env)
}

func setupContainer(name, rootfs string) error {
	// 1. Set hostname
	if err := syscall.Sethostname([]byte(name)); err != nil {
		return fmt.Errorf("failed to set hostname: %v", err)
	}

	// 2. Prepare pivot_root
	// pivot_root requires that the new root and the old root are on different filesystems.
	// We make rootfs a mount point by bind-mounting it to itself.
	// Also ensure that the mount propagation is private so we don't affect the host.
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("failed to set mount propagation to private: %v", err)
	}

	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount rootfs: %v", err)
	}

	// Create directory for the old root
	putold := filepath.Join(rootfs, ".old_root")
	if err := os.MkdirAll(putold, 0700); err != nil {
		return fmt.Errorf("failed to create .old_root: %v", err)
	}

	// 3. Mount virtual filesystems inside the new root (BEFORE pivot_root)
	mounts := []struct {
		source, target, fstype string
		flags                  uintptr
		data                   string
	}{
		{"proc", filepath.Join(rootfs, "proc"), "proc", 0, ""},
		{"sysfs", filepath.Join(rootfs, "sys"), "sysfs", 0, ""},
		{"tmpfs", filepath.Join(rootfs, "dev"), "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"},
		{"devpts", filepath.Join(rootfs, "dev/pts"), "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620,gid=5"},
		{"tmpfs", filepath.Join(rootfs, "dev/shm"), "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777"},
	}

	for _, m := range mounts {
		os.MkdirAll(m.target, 0755)
		if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, m.data); err != nil {
			// Some might fail if already mounted or unsupported, but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to mount %s: %v\n", m.target, err)
		}
	}

	// 4. pivot_root
	if err := syscall.PivotRoot(rootfs, putold); err != nil {
		return fmt.Errorf("pivot_root failed: %v", err)
	}

	// 5. Finalize root
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / failed: %v", err)
	}

	// Unmount old root and remove it
	if err := syscall.Unmount("/.old_root", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount .old_root failed: %v", err)
	}
	os.Remove("/.old_root")

	return nil
}

// Stop terminates a container process.
func Stop(rootDir string) error {
	pidPath := filepath.Join(rootDir, "pid")
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return nil
	}
	var pid int
	fmt.Sscanf(string(pidData), "%d", &pid)
	if pid <= 0 {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err == nil {
		process.Signal(syscall.SIGTERM)
		// Give it a moment to die
		for i := 0; i < 10; i++ {
			if err := process.Signal(syscall.Signal(0)); err != nil {
				break
			}
			filepath.Walk(filepath.Join(rootDir, "rootfs"), func(path string, info os.FileInfo, err error) error {
				// Try to unmount everything in rootfs to be clean
				syscall.Unmount(path, syscall.MNT_DETACH)
				return nil
			})
			os.Remove(pidPath)
			return nil
		}
		process.Kill()
	}
	os.Remove(pidPath)
	return nil
}
