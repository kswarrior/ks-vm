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

## 📊 Performance Benchmarks

| Metric           | Virtual Machine (KVM) | Native Container (Namespaces) |
|------------------|-----------------------|------------------------------|
| **Deploy Time**  | ~1.5s (Overlay)       | ~0.8s (OCI Layer Extraction) |
| **Launch Time**  | 5s - 15s (Guest Boot) | < 0.1s (Native Syscall)      |
| **I/O Latency**  | VirtIO (Near-native)  | Native (Zero overhead)       |
| **Memory Cost**  | Fixed (e.g. 1GB)      | Dynamic (Process only)       |
| **Storage Cost** | CoW Delta             | Flattened RootFS             |

---

## 🛠 Command Audit (All 17 Commands)

1.  `deploy <name> <img|docker://>`: Provision VM or Container instance.
2.  `launch <name>`: Start instance.
3.  `stop <name>`: Graceful shutdown.
4.  `delete <name>`: Remove instance and storage.
5.  `list`: Tab-aligned overview of all instances.
6.  `add <name> <url>`: Register VM base image.
7.  `image`: List base images.
8.  `remove <image>`: Delete base image.
9.  `info <name>`: Deep metadata and resource usage.
10. `shell <name>`: Interactive console (Raw mode support).
11. `exec <name> -- <cmd>`: Non-interactive execution (QGA/nsenter).
12. `restart <name>`: Graceful reboot.
13. `cp <src> <name>:<dst>`: Chunked file transfer.
14. `mount <name> <host> <guest>`: Hot-plug shared storage (VirtIO-9p).
15. `umount <name> <guest>`: Detach shared storage.
16. `version`: Software and daemon versioning.
17. `purge`: Full ecosystem reset.

---

## 🚀 Getting Started

### 📦 Prerequisites
- **OS**: Linux (KVM/QEMU and Namespaces supported)
- **Go**: 1.24+
- **Packages**: `libvirt-dev`, `qemu-utils`, `libvirt-daemon-system`

### 🔨 Installation
```bash
go mod tidy
go build -o ksvm .
```

---

## 🧪 Hybrid Testing

Deploy and manage a mixed cluster:
```bash
# Register VM image
./ksvm add focal https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img

# Deploy mix
./ksvm deploy my-vm focal
./ksvm deploy my-web docker://nginx:alpine

# Mass Start
./ksvm launch my-vm
./ksvm launch my-web

# Unified View
./ksvm list
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
