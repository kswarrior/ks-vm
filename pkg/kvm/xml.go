package kvm

import (
	"libvirt.org/go/libvirtxml"
)

// SharedFS holds configuration for a shared filesystem.
type SharedFS struct {
	SourceDir string
	TargetDir string
}

// VMConfig holds the configuration for a virtual machine.
type VMConfig struct {
	Name              string
	MemoryMB          uint
	CPUs              uint
	DiskPath          string
	ConfigDrivePath   string
	SharedFilesystems []SharedFS
	Type              string // "kvm" or "qemu"
}

// GenerateDomainXML generates the libvirt XML for a domain.
func GenerateDomainXML(config VMConfig) (string, error) {
	if config.Type == "" {
		config.Type = "kvm"
	}
	domain := &libvirtxml.Domain{
		Type: config.Type,
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
			Filesystems: generateFilesystems(config.SharedFilesystems),
			Disks:       generateDisks(config),
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
			Channels: []libvirtxml.DomainChannel{
				{
					Source: &libvirtxml.DomainChardevSource{
						UNIX: &libvirtxml.DomainChardevSourceUNIX{
							Mode: "bind",
						},
					},
					Target: &libvirtxml.DomainChannelTarget{
						VirtIO: &libvirtxml.DomainChannelTargetVirtIO{
							Name: "org.qemu.guest_agent.0",
						},
					},
				},
			},
			MemBalloon: &libvirtxml.DomainMemBalloon{
				Model: "virtio",
				Stats: &libvirtxml.DomainMemBalloonStats{
					Period: 5,
				},
			},
		},
	}

	return domain.Marshal()
}

func generateFilesystems(shared []SharedFS) []libvirtxml.DomainFilesystem {
	var fss []libvirtxml.DomainFilesystem
	for _, s := range shared {
		fss = append(fss, libvirtxml.DomainFilesystem{
			AccessMode: "passthrough",
			Source: &libvirtxml.DomainFilesystemSource{
				Mount: &libvirtxml.DomainFilesystemSourceMount{
					Dir: s.SourceDir,
				},
			},
			Target: &libvirtxml.DomainFilesystemTarget{
				Dir: s.TargetDir,
			},
		})
	}
	return fss
}

func uintPtr(u uint) *uint {
	return &u
}

func generateDisks(config VMConfig) []libvirtxml.DomainDisk {
	disks := []libvirtxml.DomainDisk{
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
	}

	if config.ConfigDrivePath != "" {
		disks = append(disks, libvirtxml.DomainDisk{
			Device: "cdrom",
			Driver: &libvirtxml.DomainDiskDriver{
				Name: "qemu",
				Type: "raw",
			},
			Source: &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					File: config.ConfigDrivePath,
				},
			},
			Target: &libvirtxml.DomainDiskTarget{
				Dev: "sda",
				Bus: "sata",
			},
			ReadOnly: &libvirtxml.DomainDiskReadOnly{},
		})
	}
	return disks
}
