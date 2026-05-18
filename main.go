package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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

func init() {
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
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

		fmt.Printf("%-20s %-10s %-20s\n", "NAME", "STATUS", "IP ADDRESSES")
		for _, vm := range vms {
			ips := strings.Join(vm.IPs, ", ")
			if ips == "" {
				ips = "-"
			}
			fmt.Printf("%-20s %-10s %-20s\n", vm.Name, vm.Status, ips)
		}
	},
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
		if err := manager.Deploy(name, image); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("VM %s deployed successfully.\n", name)
	},
}
