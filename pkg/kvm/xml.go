package kvm

import (
	"libvirt.org/go/libvirtxml"
)

// VMConfig holds the configuration for a virtual machine.
type VMConfig struct {
	Name     string
	MemoryMB uint
	CPUs     uint
	DiskPath string
}

// GenerateDomainXML generates the libvirt XML for a domain.
func GenerateDomainXML(config VMConfig) (string, error) {
	domain := &libvirtxml.Domain{
		Type: "kvm",
		Name: config.Name,
		Memory: &libvirtxml.DomainMemory{
			Value: config.MemoryMB,
			Unit:  "MiB",
		},
		VCPU: &libvirtxml.DomainVCPU{
			Value: config.CPUs,
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Arch:    "x86_64",
				Machine: "q35",
				Type:    "hvm",
			},
		},
		Features: &libvirtxml.DomainFeatureList{
			ACPI: &libvirtxml.DomainFeature{},
			APIC: &libvirtxml.DomainFeatureAPIC{},
		},
		OnPoweroff: "destroy",
		OnReboot:   "restart",
		OnCrash:    "destroy",
		Devices: &libvirtxml.DomainDeviceList{
			Disks: []libvirtxml.DomainDisk{
				{
					Device: "disk",
					Driver: &libvirtxml.DomainDiskDriver{
						Name: "qemu",
						Type: "qcow2",
					},
					Source: &libvirtxml.DomainDiskSource{
						File: &libvirtxml.DomainDiskSourceFile{
							File: config.DiskPath,
						},
					},
					Target: &libvirtxml.DomainDiskTarget{
						Dev: "vda",
						Bus: "virtio",
					},
				},
			},
			Interfaces: []libvirtxml.DomainInterface{
				{
					Source: &libvirtxml.DomainInterfaceSource{
						Network: &libvirtxml.DomainInterfaceSourceNetwork{
							Network: "default",
						},
					},
					Model: &libvirtxml.DomainInterfaceModel{
						Type: "virtio",
					},
				},
			},
			Consoles: []libvirtxml.DomainConsole{
				{
					Target: &libvirtxml.DomainConsoleTarget{
						Type: "serial",
						Port: uintPtr(0),
					},
				},
			},
		},
	}

	return domain.Marshal()
}

func uintPtr(u uint) *uint {
	return &u
}
