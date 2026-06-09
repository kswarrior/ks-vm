# KS VM ⚡ Triple-Stack Hybrid Virtualization

KS VM is a professional-grade, high-performance hybrid cloud manager built in Go. It offers a single, unified pane of glass to manage **KVM/QEMU Virtual Machines**, **LXD System Containers**, and **Docker Application Containers**.

![KS VM Dashboard](https://raw.githubusercontent.com/ks-vm/ks-vm/main/screenshots/dashboard.png)

## 🌟 Why KS VM is the Best?

In the evolving landscape of cloud infrastructure, KS VM stands as the definitive "Triple Threat" management solution:

1.  **True Hybrid Flexibility:** Stop switching between `virt-manager`, `lxc`, and `docker` CLI. KS VM unifies the three most powerful Linux virtualization technologies into one responsive dashboard.
2.  **Deterministic State Management:** Every instance, whether a VM or a container, is tracked via a persistent `meta.json` source-of-truth. This ensures lifecycle actions (Start, Stop, Exec) are routed with 100% accuracy.
3.  **Resilient Out-of-Band Access:** Never get locked out. If the QEMU Guest Agent is missing or unresponsive, KS VM automatically triggers an **Automated Serial Console Injection** with credentials login, providing shell access where others fail.
4.  **Optimistic, High-Density UX:** Our frontend utilizes an **Optimistic UI pattern** with surgical DOM updates. Experience instant feedback on deployments and flicker-free resource monitoring even on high-latency connections.
5.  **Interactive Session Isolation:** The integrated **xterm.js terminal** provides independent, persistent sessions. Switch between a Docker microservice and a full KVM server without losing your command history or output buffer.
6.  **Single-Binary Portability:** Compiled with `go:embed`, KS VM is a self-contained ecosystem. No external web server or asset directory required—just one binary to rule them all.

## 🚀 Key Features

- **Triple Compute Backend:** Native integration for KVM/QEMU, LXC/LXD, and Docker.
- **Modern Glassmorphism UI:** A stunning, professional dashboard designed for both desktop power-users and mobile administrators.
- **Real-Time Observability:** High-fidelity host monitoring using **Chart.js** for CPU Load (1/5/15m), RAM distribution, Disk usage, and Network throughput.
- **Interactive xterm.js Terminal:** Full ANSI color support, command history (Up/Down), local echo, and native keyboard handling.
- **Persistent SSH Infrastructure:** One-click reverse SSH tunnel setup with custom port and token/URL configuration.
- **Gateway Multiplexer:** A built-in TCP gateway for routing traffic to internal instances based on custom headers.

## 🛠 Installation

### Prerequisites (Ubuntu/Debian)
```bash
sudo apt-get update
sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system xorriso lxd docker.io
```

### Build from Source
```bash
git clone https://github.com/ks-vm/ks-vm.git
cd ks-vm
./build.sh
```

## 📖 CLI Reference

| Command | Usage | Description |
| :--- | :--- | :--- |
| `deploy` | `ksvm deploy <name> <image> [flags]` | Deploy a new VM, LXD, or Docker instance. |
| `launch` | `ksvm launch <name>` | Start a stopped instance. |
| `stop` | `ksvm stop <name>` | Gracefully shut down an instance. |
| `restart` | `ksvm restart <name>` | Reboot an instance. |
| `shell` | `ksvm shell <name>` | Native interactive terminal session. |
| `exec` | `ksvm exec <name> -- <cmd>` | Run a command inside the guest. |
| `list` | `ksvm list` | List all compute instances across all drivers. |
| `add` | `ksvm add <name> <source>` | Register a base OS image or Docker tag. |
| `image` | `ksvm image` | List registered OS images. |
| `info` | `ksvm info <name>` | Fetch detailed telemetry and metadata. |
| `daemon` | `ksvm daemon [flags]` | Start the Web UI and Gateway. |
| `purge` | `ksvm purge` | Wipe all instances and data. |

### Global Flags
- `-P, --port`: Multi-service port mapping (e.g., `w:8080 m:5050`).
- `--user, --pass`: Master credentials for the Daemon/Web UI Basic Auth.

## 🖥 Web Interface

The KS VM dashboard is optimized for efficiency and aesthetics.

### Real Shell Experience
The **RUN CODE** feature provides a persistent shell. It handles nested shell characters and complex escaping, ensuring commands like `mkdir -p /path/to/dir` work flawlessly across all architectures.

### System Dashboard
Track your host's health with sub-second precision. Includes Kernel version, Platform identification, Active instance counts, and detailed Uptime tracking.

## 🏗 Architecture

KS VM is designed for modularity and extreme speed:
- **KVM Driver:** Leverages `libvirt` for hardware-accelerated virtualization.
- **LXD Driver:** High-density system containers via Unix socket REST API.
- **Docker Driver:** Standardized application containers via Docker Engine API.
- **REST API:** High-concurrency Gin-powered backend.
- **Deterministic Metadata:** Instance identity persists across reboots via `meta.json`.

---
*Built for the next generation of cloud infrastructure and DevOps excellence.*
