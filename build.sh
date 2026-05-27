#!/bin/bash
set -e

echo "ksvm ⚡ Hybrid Virtualization Builder"
echo "--- Installing dependencies..."
sudo apt-get update && sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system xorriso snapd

# Install LXD if not present
if ! command -v lxc &> /dev/null; then
    echo "--- Attempting to install LXD via snap..."
    sudo snap install lxd || echo "Warning: Snap install failed. Please install LXD manually if needed."
fi

# Ensure LXD is initialized
if command -v lxd &> /dev/null; then
    if ! sudo lxd init --auto &> /dev/null; then
        echo "--- LXD already initialized or init failed (non-critical)"
    fi
fi

# Add current user to lxd group to avoid sudo requirements for the socket
sudo usermod -aG lxd $USER || true

echo "--- Tidying Go modules..."
go mod tidy

echo "--- Formatting source code..."
go fmt ./...

echo "--- Compiling ksvm binary..."
go build -o ksvm .

echo "--- Finalizing..."
sudo chmod +x ksvm

echo "Success! Run './ksvm daemon --user admin --pass admin' to start the management panel."
