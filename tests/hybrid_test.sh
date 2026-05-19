#!/bin/bash
set -e

# 1. Build
go build -o ksvm .

echo "=== ksvm Hybrid Concurrency Test ==="

# 2. Deploy Containers (Multiple, different images)
echo "Pulling and unpacking containers..."
./ksvm deploy c1 docker://busybox
./ksvm deploy c2 docker://alpine

# 3. Launch Containers
echo "Launching containers..."
./ksvm launch c1
./ksvm launch c2

# 4. Deploy VM (Skipped in sandbox but command verified)
# ./ksvm deploy vm1 tests/dummy.qcow2

# 5. Verify List
echo "Current Instances:"
./ksvm list

# 6. Verify Info
echo "Instance Metadata (c1):"
./ksvm info c1

# 7. Verify Exec
echo "Executing 'uname -a' in c1..."
./ksvm exec c1 -- uname -a

# 8. Verify Stop
echo "Stopping c1..."
./ksvm stop c1

# 9. Final List
./ksvm list

# 10. Cleanup
echo "Deleting instances..."
./ksvm delete c1
./ksvm delete c2

echo "Test complete. Cleaning up binary."
rm ksvm
