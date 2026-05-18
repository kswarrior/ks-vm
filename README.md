# ksvm - Native Linux Virtualization Manager

ksvm is a custom, high-performance virtual machine manager CLI tool built from scratch in Go. It talks directly to the local libvirt daemon using KVM and QEMU, providing a native experience for managing virtualization on Linux systems.

## Why ksvm?

- **Native & Direct**: Unlike tools that wrap other CLI utilities, ksvm communicates directly with `libvirtd` via official Go bindings.
- **Optimized for Speed**: Uses QCOW2 copy-on-write overlays and backing chains for near-instant VM deployment.
- **Expert Interaction**: Leverages the QEMU Guest Agent for seamless command execution and file transfers without needing SSH.
- **Cyberpunk UI**: Includes a responsive, premium Web UI designed with a modern aesthetic for those who prefer graphical management.
- **Multiplexed Gateway**: Built-in high-speed data multiplexer for routing guest streams and internal services.

## Core Features

- **Lifecycle Management**: Deploy, Launch, Stop, Restart, and Delete VMs with ease.
- **Image Engine**: Register cloud images from URLs or local paths with download progress tracking.
- **Guest Interaction**: Run commands (`exec`), copy files (`cp`), and access the serial console (`shell`).
- **Shared Storage**: Hot-plug host directories into running guests using VirtIO-9p.
- **Daemon Mode**: Run as a service providing a REST API and Cyberpunk Web Dashboard.
- **System Maintenance**: Version reporting and full environment purge capability.

## Installation

### Dependencies
- Go 1.25+
- libvirt-dev
- qemu-utils
- libvirt-daemon-system

```bash
sudo apt-get update
sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system
```

### Build
```bash
go mod tidy
go build -o ksvm .
```

## Usage

### CLI Commands
```bash
./ksvm deploy <name> <image>    # Deploy a VM instantly
./ksvm launch <name>           # Start a VM
./ksvm stop <name>             # Graceful shutdown
./ksvm exec <name> -- <cmd>    # Run command in guest
./ksvm shell <name>            # Interactive console
./ksvm daemon --port w:8080    # Start Web UI
```

### Web API Examples

#### List Instances (Node.js)
```javascript
const http = require('http');
http.get('http://localhost:8080/api/v1/instances', (res) => {
    res.on('data', d => process.stdout.write(d));
});
```

#### Get Info (Go)
```go
resp, _ := http.Get("http://localhost:8080/api/v1/info/my-vm")
```

## Architecture

- **main.go**: CLI entry point and routing.
- **pkg/kvm**: Core virtualization engine (XML generation, libvirt actions).
- **pkg/api**: RESTful API handlers.
- **pkg/daemon**: Concurrent server management and gateway mux.
- **pkg/web**: Embedded Cyberpunk Web UI assets.

## License
MIT
