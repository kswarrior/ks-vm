package kvm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"libvirt.org/go/libvirt"
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
	// 1. Ensure directories exist
	instancesDir := filepath.Join(BaseDir, "instances", name)
	if err := os.MkdirAll(instancesDir, 0755); err != nil {
		return fmt.Errorf("failed to create instances directory: %v", err)
	}

	diskPath := filepath.Join(instancesDir, "disk.qcow2")

	// 2. Create QCOW2 overlay
	// qemu-img create -f qcow2 -b <baseImage> -F qcow2 <diskPath>
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", baseImage, "-F", "qcow2", diskPath)
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
	Name string
	Status string
	IPs []string
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
