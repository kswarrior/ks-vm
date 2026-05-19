package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Run starts a container using namespaces and chroot.
func Run(name, rootDir string, args []string) error {
	rootfs := filepath.Join(rootDir, "rootfs")

	// We re-execute the current process to set up namespaces
	if os.Getenv("KSVM_IN_CONTAINER") == "" {
		cmd := exec.Command("/proc/self/exe", append([]string{"internal-run", name, rootDir}, args...)...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS,
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "KSVM_IN_CONTAINER=1")

		// Record the PID for management
		if err := cmd.Start(); err != nil {
			return err
		}

		pidPath := filepath.Join(rootDir, "pid")
		os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)

		return nil
	}

	// Inside the namespace
	if err := syscall.Sethostname([]byte(name)); err != nil {
		return fmt.Errorf("failed to set hostname: %v", err)
	}
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("failed to chroot: %v", err)
	}
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir: %v", err)
	}

	// Mount proc
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %v", err)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Stop terminates a container process.
func Stop(rootDir string) error {
	pidData, err := os.ReadFile(filepath.Join(rootDir, "pid"))
	if err != nil {
		return err
	}
	var pid int
	fmt.Sscanf(string(pidData), "%d", &pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
