# ksvm: Native Linux Hybrid Virtualization Manager 🚀

**ksvm** is a high-performance, custom-built management suite for Linux that seamlessly blends **Virtual Machines (KVM/QEMU)** and **Native Containers (Linux Namespaces)** into a single, cohesive experience.

Built from scratch in Go, ksvm avoids heavy external dependencies by communicating directly with the `libvirt` daemon for VMs and utilizing native system calls for its lightweight container runtime.

---

## 💎 Why ksvm is the Best

- **Hybrid Orchestration**: Deploy a full-weight Virtual Machine or a lightweight OCI Container with the exact same command.
- **Zero-Latency VM Deployment**: Uses QCOW2 backing chains for instant VM provisioning.
- **Native Container Runtime**: Implements a custom OCI unpacker and namespace-based runner (PID, NS, UTS) for ultra-lightweight process isolation.
- **Agent-First Interaction**: Full QEMU Guest Agent integration for command execution and file transfers without SSH.
- **Cyberpunk Dashboard**: A premium, responsive Web UI featuring real-time monitoring and glassmorphic design.
- **Multiplexed Gateway**: Built-in high-speed TCP proxy for routing guest streams through a single host entry point.

---

## 🛠 Core Features

- ✅ **Unified Lifecycle**: Deploy, Launch, Stop, Suspend, Resume, and Delete for both VMs and Containers.
- ✅ **Image Engine**:
    - VM: Register .qcow2 cloud images from URLs or local paths.
    - Container: Pull directly from Docker registries (e.g., `docker://nginx:latest`).
- ✅ **Guest Interaction**: Exec, Copy, and Interactive Shell with raw terminal mode.
- ✅ **Shared Storage**: Hot-plug host directories into guests via VirtIO-9p.
- ✅ **Daemon Mode**: Multi-service engine providing a REST API and Cyberpunk Dashboard.

---

## 🚀 Getting Started

### 📦 Prerequisites
- **OS**: Linux (KVM/QEMU and Namespaces supported)
- **Go**: 1.24+
- **Packages**: `libvirt-dev`, `qemu-utils`, `libvirt-daemon-system`

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system
```

### 🔨 Installation
```bash
go mod tidy
go build -o ksvm .
```

---

## 📖 Usage Guide

### Deploying a VM
```bash
./ksvm add ubuntu https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
./ksvm deploy my-vm ubuntu
```

### Deploying a Container
```bash
./ksvm deploy my-container docker://nginx:alpine
```

### Management
```bash
./ksvm list                  # Show all instances (VMs & Containers)
./ksvm info my-container     # Deep metadata
./ksvm stop my-vm            # Graceful shutdown
./ksvm daemon --port w:8080  # Launch Cyberpunk Web UI
```

---

## 🏗 Architecture

- **main.go**: Intelligent CLI routing and entry point.
- **pkg/kvm**: Native libvirt virtualization engine.
- **pkg/container**: OCI unpacker and Linux namespace runtime.
- **pkg/api**: High-performance REST API (Gin).
- **pkg/daemon**: Concurrent server management and gateway mux.
- **pkg/web**: Embedded Cyberpunk Web UI assets.

---

## 📜 License
MIT
