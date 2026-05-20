#!/bin/bash
set -e

echo "ksvm ⚡ Hybrid Virtualization Builder"
echo "--- Installing dependencies..."
sudo apt-get update && sudo apt-get install -y libvirt-dev qemu-utils libvirt-daemon-system xorriso

echo "--- Tidying Go modules..."
go mod tidy

echo "--- Formatting source code..."
go fmt ./...

echo "--- Compiling ksvm binary..."
go build -o ksvm .

echo "--- Finalizing..."
sudo chmod +x ksvm

echo "Success! Run './ksvm daemon --user admin --pass admin' to start the management panel."
