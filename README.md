# KS VM ⚡ Hybrid Virtualization & Monitoring Stack

KS VM is a professional-grade, hybrid cloud management tool built in Go. It offers a unified platform to manage high-performance **KVM/QEMU Virtual Machines** alongside ultra-lightweight **LXC/LXD System Containers**.

![KS VM Dashboard](https://raw.githubusercontent.com/ks-vm/ks-vm/main/screenshots/dashboard.png)

## 🚀 Key Features

- **Hybrid Compute Backend:** Simultaneously manage full hardware-virtualized VMs and kernel-isolated LXD containers.
- **Real-Time Observability:** Professional system dashboard featuring **Chart.js** for live CPU, RAM, Disk, and Network monitoring of the host VPS.
- **Resilient Management:** Automated **Serial Console Fallback** for VM execution when the QEMU Guest Agent is unavailable.
- **Persistent SSH Infrastructure:** Integrated setup for long-running reverse SSH tunnels that survive session termination.
- **Rapid Provisioning:** Instant VM deployment using QCOW2 backing chains and multi-arch OCI/Docker image support for containers.
- **Gateway Multiplexer:** Built-in TCP gateway for routing external traffic to internal instances using custom headers.

## 🛠 Installation

### Prerequisites (Ubuntu/Debian)
```bash
sudo apt-get update
sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system xorriso lxd
```

### Build from Source
```bash
git clone https://github.com/ks-vm/ks-vm.git
cd ks-vm
./build.sh
```

## 📖 CLI Reference

KS VM provides a powerful command-line interface for local and remote automation.

| Command | Usage | Description |
| :--- | :--- | :--- |
| `deploy` | `ksvm deploy <name> <image> [flags]` | Deploy a new instance. |
| `launch` | `ksvm launch <name>` | Start a stopped instance. |
| `stop` | `ksvm stop <name>` | Gracefully shut down an instance. |
| `restart` | `ksvm restart <name>` | Reboot an instance. |
| `shell` | `ksvm shell <name>` | Interactive terminal session. |
| `exec` | `ksvm exec <name> -- <cmd>` | Run a command inside the guest. |
| `list` | `ksvm list` | List all compute instances. |
| `add` | `ksvm add <name> <source>` | Register a base OS image. |
| `image` | `ksvm image` | List registered OS images. |
| `info` | `ksvm info <name>` | Fetch detailed telemetry metadata. |
| `daemon` | `ksvm daemon [flags]` | Start the Web UI and Gateway. |
| `purge` | `ksvm purge` | Wipe all instances and data. |

### Global Flags
- `-P, --port`: Multi-service port mapping (e.g., `w:8080 m:5050`).
- `--user, --pass`: Master credentials for the Daemon/Web UI.

## 🖥 Web Interface

The KS VM Daemon includes a professional, responsive glassmorphic dashboard accessible via any modern browser.

### Host Monitoring
- **Live CPU Graph:** Real-time usage tracking across all cores.
- **RAM/Disk Gauges:** Visual breakdown of used vs. available resources.
- **Network Throughput:** Dynamic RX/TX speed visualization in KB/s.

### SSH setup
When a VM is running, click **SSH TOKEN** to automatically initialize a persistent reverse tunnel. You can monitor the setup progress in real-time through the integrated terminal view.

## 🏗 Architecture

KS VM is designed for modularity and speed:
- **LXD Backend:** Communicates via local Unix socket for industry-standard containerization.
- **libvirt/KVM:** High-performance hardware virtualization for full operating systems.
- **REST API:** Gin-powered v1 API for seamless front-end integration.
- **Assets:** Fully embedded HTML/CSS/JS for single-binary portability.

---
*Developed by Systems Engineers for high-availability cloud infrastructure.*
