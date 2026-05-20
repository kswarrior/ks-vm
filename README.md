# ksvm ⚡ Hybrid Virtualization Manager

ksvm is a high-performance, native Linux virtualization manager built from scratch in Go. It provides a unified interface for managing both **KVM/QEMU Virtual Machines** and **Native Linux Containers** (using namespaces and chroot).

## 🚀 Key Features

- **Hybrid Management:** Seamlessly deploy and control VMs and Containers from a single CLI.
- **Rapid VM Deployment:** Uses QCOW2 backing chains for instant deployment (0 seconds on disk).
- **OCI Container Support:** Pulls and unpacks Docker/OCI images directly into isolated namespaces.
- **Cyberpunk Web UI:** A beautiful, responsive dashboard for real-time monitoring and control.
- **TCP Gateway Multiplexer:** Route external traffic to internal instances using custom headers.
- **Guest Interaction:** Integrated 'exec', 'cp', and 'mount' commands via QEMU Guest Agent.
- **Cloud-Init Integration:** Injects user credentials and configuration during VM deployment.

## 🛠 Installation

### Prerequisites

Ensure you have the following dependencies installed on your Linux host:

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system xorriso
```

### Build from Source

```bash
git clone https://github.com/ks-vm/ks-vm.git
cd ks-vm
go build -o ksvm .
```

## 📖 Usage

### VM Lifecycle
```bash
# Register a base image
./ksvm add focal https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img

# Deploy a new VM with credentials
./ksvm deploy my-vm focal --user admin --pass secret

# Launch and connect
./ksvm launch my-vm
./ksvm shell my-vm
```

### Container Lifecycle
```bash
# Deploy an Nginx container from Docker Hub
./ksvm deploy my-web docker://nginx:alpine

# Launch and connect
./ksvm launch my-web
./ksvm shell my-web
```

### Daemon & Web UI
```bash
# Start the daemon on port 8080 (Web) and 5050 (Gateway)
./ksvm daemon -P "w:8080 m:5050"
```

## 📊 Performance Benchmarks

| Action | VM (KVM) | Container (Namespaces) |
| :--- | :--- | :--- |
| **Deployment Time** | < 1s (QCOW2 Overlay) | < 3s (Network Dependent) |
| **Startup Time** | ~10s (Full OS Boot) | < 0.1s (Process Exec) |
| **Storage Overhead** | < 1MB (Delta) | Layer Dependent |
| **Isolation** | High (Hardware Virtualization) | Medium (Namespaces/chroot) |

## 🏗 Architecture

ksvm uses a modular engine architecture:
- **pkg/kvm:** Interfaces with libvirt via native CGo bindings.
- **pkg/container:** A custom OCI layer puller and namespace-based process runner.
- **pkg/daemon:** A Gin-based REST API and an embedded glassmorphic Web UI.
- **pkg/daemon/mux:** A TCP multiplexer for routing guest streams.

---
*Created by expert Systems Engineers for the next generation of cloud infrastructure.*
