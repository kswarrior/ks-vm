package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			Setsid:     true, // Detach from parent
		}
		cmd.Env = append(os.Environ(), "KSVM_IN_CONTAINER=1")

		// Handle background execution
		if os.Getenv("KSVM_BG") == "1" {
			logPath := filepath.Join(rootDir, "container.log")
			logFile, err := os.Create(logPath)
			if err != nil {
				return err
			}
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			cmd.Stdin = nil // Detach Stdin
		} else {
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		// Record the PID for management
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

	// Set a basic PATH if none exists
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

// Stop terminates a container process.
func Stop(rootDir string) error {
	pidPath := filepath.Join(rootDir, "pid")
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already stopped or never started
		}
		return err
	}
	var pid int
	if _, err := fmt.Sscanf(string(pidData), "%d", &pid); err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// On Unix, FindProcess always succeeds and returns a Process object
	// with the given pid. We should check if the process actually exists.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		os.Remove(pidPath)
		return nil
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	os.Remove(pidPath)
	return nil
}
