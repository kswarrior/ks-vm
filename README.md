# KS VM ⚡ Hybrid Cloud & Container Stack

KS VM is a professional-grade, high-performance virtualization and container management platform built in Go. It provides a unified, glassmorphic interface to manage the full spectrum of compute: **KVM Virtual Machines**, **LXD System Containers**, and **Docker Application Containers**.

![KS VM Dashboard](https://raw.githubusercontent.com/ks-vm/ks-vm/main/screenshots/dashboard.png)

## 🌟 Why KS VM is the Best?

In a crowded market of virtualization tools, KS VM stands out as the ultimate "Triple Threat" for systems engineers and developers:

1.  **Unified Hybrid Architecture:** Stop switching between tools. Manage full hardware-isolated VMs (KVM), high-density system containers (LXD), and standardized application microservices (Docker) from a single glassmorphic pane.
2.  **Deterministic Reliability:** Unlike other wrappers, KS VM utilizes a persistent `meta.json` source-of-truth for every instance. Whether you're starting, stopping, or executing code, the system knows exactly how to route the command to the correct provider.
3.  **Intelligent Execution Fallback:** Never lose access to your VMs. If the QEMU Guest Agent fails, KS VM automatically falls back to an advanced **Serial Console Injection** mechanism with automated login, ensuring you always have a shell.
4.  **Zero-Dependency Portability:** KS VM is compiled into a single binary. All frontend assets (HTML/CSS/JS) are embedded via `go:embed`. Deploying your cloud control panel is as simple as copying one file.
5.  **Professional-Grade UX:** Experience ultra-fast, flicker-free updates with our **Optimistic UI** pattern. Actions like "Deploy" or "Stop" reflect in the UI instantly while the backend handles the heavy lifting.

## 🚀 Key Features

- **Triple Compute Backend:** Simultaneously manage KVM/QEMU, LXC/LXD, and Docker containers.
- **Interactive xterm.js Terminal:** A "real shell" experience inside your browser. Supports ANSI colors, command history (Up/Down arrows), local echo, and keyboard shortcuts (Ctrl+C, Backspace).
- **Modern Glassmorphism UI:** A stunning, high-density dashboard that is fully mobile-responsive. Swipe through metrics on your phone or manage clusters on your desktop.
- **Real-Time Observability:** Professional host monitoring using **Chart.js**. Track CPU load averages, RAM distribution, Disk I/O, and Network throughput (RX/TX) in real-time.
- **Persistent SSH Infrastructure:** One-click setup for reverse SSH tunnels using custom ports and tokens, designed to survive session timeouts.
- **Gateway Multiplexer:** Built-in TCP gateway for routing external traffic to internal instances using custom headers.

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

The KS VM Daemon includes a professional, responsive glassmorphic dashboard.

### Real Shell Experience
The **RUN CODE** action opens a persistent terminal powered by `xterm.js`. It features session isolation, meaning you can switch between instances without losing your command history or output buffer.

### Host Monitoring
- **CPU:** Real-time percentage across all logical cores.
- **RAM/Disk:** Visual doughnut/pie charts for precise resource allocation.
- **Load Average:** 1, 5, and 15-minute system load metrics.

## 🏗 Architecture

KS VM is designed for modularity and speed:
- **KVM Driver:** High-performance hardware virtualization via `libvirt`.
- **LXD Driver:** System containerization via Unix socket API.
- **Docker Driver:** Application containerization via Docker socket.
- **REST API:** High-concurrency Gin-powered v1 API.
- **Deterministic Metadata:** Instance state managed via early-persistence `meta.json`.

---
*Developed for high-availability cloud infrastructure and modern DevOps workflows.*
