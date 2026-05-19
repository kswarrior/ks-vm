package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"ksvm/pkg/container"
	"ksvm/pkg/daemon"
	"ksvm/pkg/kvm"
)

var rootCmd = &cobra.Command{
	Use:   "ksvm",
	Short: "ksvm is a custom virtual machine manager CLI tool",
	Long:  `ksvm is a native Linux virtualization manager that talks directly to the local libvirt daemon using KVM and QEMU.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var shellToken string
var portMap string

var deployUser string
var deployPass string

func init() {
	rootCmd.PersistentFlags().StringVarP(&portMap, "port", "P", "", "Multi-service port mapping (e.g. w:8080 m:5050)")

	deployCmd.Flags().StringVarP(&deployUser, "user", "u", "", "Default guest user to create via Cloud-Init")
	deployCmd.Flags().StringVarP(&deployPass, "pass", "p", "", "Default guest password via Cloud-Init")

	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(imageCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(cpCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(umountCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(purgeCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(suspendCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(internalRunCmd)

	shellCmd.Flags().StringVarP(&shellToken, "token", "t", "", "Custom session token for Web UI hook")
	purgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "Force purge without confirmation")
}

var deployCmd = &cobra.Command{
	Use:   "deploy <name> <image>",
	Short: "Deploy a new VM from a base image",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		image := args[1]

		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		fmt.Printf("Deploying VM %s with image %s...\n", name, image)
		opts := kvm.DeployOptions{
			User:     deployUser,
			Password: deployPass,
		}
		if err := manager.Deploy(name, image, opts); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("VM %s deployed successfully.\n", name)
	},
}

var launchCmd = &cobra.Command{
	Use:   "launch <name>",
	Short: "Start a stopped VM instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Launch(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("VM %s started.\n", name)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Send a graceful ACPI shutdown signal to the VM",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Stop(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Shutdown signal sent to VM %s.\n", name)
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Stop and destroy the libvirt domain mapping",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Delete(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("VM %s deleted.\n", name)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VMs, their current states, and IP addresses",
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		vms, err := manager.List()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tIP ADDRESSES")
		for _, vm := range vms {
			ips := strings.Join(vm.IPs, ", ")
			if ips == "" {
				ips = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", vm.Name, vm.Type, vm.Status, ips)
		}
		w.Flush()
	},
}

var addCmd = &cobra.Command{
	Use:   "add <name> <url_or_path>",
	Short: "Register a base cloud image",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.AddImage(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Image %s registered.\n", args[0])
	},
}

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "List all registered base images",
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		images, err := manager.ListImages()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSIZE\tADDED AT")
		for _, img := range images {
			fmt.Fprintf(w, "%s\t%d\t%s\n",
				img.Name,
				img.Size,
				img.AddedAt.Format("2006-01-02 15:04"),
			)
		}
		w.Flush()
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a base image from the local storage pool",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.RemoveImage(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Image %s removed.\n", args[0])
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Fetch and display detailed VM resource metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		info, err := manager.Info(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 15, 1, 2, ' ', 0)
		fmt.Fprintf(w, "NAME:\t%s\n", info.Name)
		fmt.Fprintf(w, "TYPE:\t%s\n", info.Type)
		fmt.Fprintf(w, "STATUS:\t%s\n", info.Status)
		fmt.Fprintf(w, "CPUs:\t%d\n", info.CPUs)
		fmt.Fprintf(w, "MEMORY:\t%d MiB\n", info.MemoryMB)
		fmt.Fprintf(w, "DISK USAGE:\t%d bytes\n", info.DiskUsage)
		fmt.Fprintf(w, "IPs:\t%s\n", strings.Join(info.IPs, ", "))
		w.Flush()
	},
}

var shellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Establish an interactive terminal session inside the guest VM",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if shellToken != "" {
			fmt.Printf("Establishing session using token: %s\n", shellToken)
			// TODO: Bridge this into a permanent web WebSocket terminal connection.
			// The token will be used to authenticate the stream access via the web adapter.
			fmt.Printf("Stream path: /var/run/libvirt/ksvm-%s.sock\n", args[0])
			return
		}

		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Shell(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	},
}

var execCmd = &cobra.Command{
	Use:   "exec <name> -- <command> [args...]",
	Short: "Run a non-interactive shell command inside the guest VM",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		resp, err := manager.Exec(args[0], args[1:])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(resp)
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart <name>",
	Short: "Reboot the guest VM gracefully",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Restart(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("VM %s restarted.\n", args[0])
	},
}

var cpCmd = &cobra.Command{
	Use:   "cp <local_file> <name>:<guest_path>",
	Short: "Copy a file from host to guest",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		parts := strings.Split(args[1], ":")
		if len(parts) != 2 {
			fmt.Println("Error: target must be in format <name>:<guest_path>")
			return
		}
		name, guestPath := parts[0], parts[1]

		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Copy(name, args[0], guestPath); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("File %s copied to %s:%s\n", args[0], name, guestPath)
	},
}

var mountCmd = &cobra.Command{
	Use:   "mount <name> <local_path> <vm_path>",
	Short: "Share a host directory with the guest",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Mount(args[0], args[1], args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Directory %s mounted to VM %s at %s\n", args[1], args[0], args[2])
	},
}

var umountCmd = &cobra.Command{
	Use:   "umount <name> <vm_path>",
	Short: "Detach a shared directory from the VM",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Umount(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Directory %s unmounted from VM %s\n", args[1], args[0])
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		v, err := manager.Version()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("ksvm version:    %s\n", v.KSVM)
		fmt.Printf("libvirt version: %s\n", v.Libvirt)
		fmt.Printf("QEMU version:    %s\n", v.QEMU)
	},
}

var purgeForce bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run ksvm in daemon mode with Web UI and Gateway",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := parsePortMap(portMap)
		if cfg.WebPort == "" {
			cfg.WebPort = "8080"
		}
		if cfg.MuxPort == "" {
			cfg.MuxPort = "5050"
		}

		if err := daemon.Start(cfg); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func parsePortMap(pm string) daemon.Config {
	cfg := daemon.Config{}
	parts := strings.Fields(pm)
	for _, p := range parts {
		kv := strings.Split(p, ":")
		if len(kv) == 2 {
			switch kv[0] {
			case "w":
				cfg.WebPort = kv[1]
			case "m":
				cfg.MuxPort = kv[1]
			}
		}
	}
	return cfg
}

var suspendCmd = &cobra.Command{
	Use:   "suspend <name>",
	Short: "Pause a running instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Suspend(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Instance %s suspended.\n", args[0])
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume <name>",
	Short: "Continue a suspended instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		if err := manager.Resume(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Instance %s resumed.\n", args[0])
	},
}

var internalRunCmd = &cobra.Command{
	Use:    "internal-run <name> <dir>",
	Hidden: true,
	Args:   cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := container.Run(args[0], args[1], args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Internal Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Reset the entire host virtualization ecosystem",
	Run: func(cmd *cobra.Command, args []string) {
		if !purgeForce {
			fmt.Print("This will destroy ALL VMs and images. Are you sure? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Purge aborted.")
				return
			}
		}

		manager, err := kvm.NewManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer manager.Close()

		fmt.Println("Purging all VMs and images...")
		if err := manager.Purge(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("Purge complete.")
	},
}
