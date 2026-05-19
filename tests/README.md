# ksvm Testing & Performance

## Multi-Instance Concurrency Test
Run the script to deploy and manage multiple containers/VMs side-by-side.
```bash
./tests/concurrency_test.sh
```

## Expected Performance Metrics

### Virtual Machines (KVM)
- **Deploy Time**: ~1-2 seconds (instant overlay creation, plus XML registration)
- **Start Time**: ~5-15 seconds (depending on guest OS boot speed)
- **CPU/RAM Overhead**: Near-native (KVM)

### Containers (Namespaces)
- **Deploy Time**: ~0.5-2 seconds (layer extraction speed)
- **Start Time**: ~0.1 seconds (instant process execution)
- **CPU/RAM Overhead**: Zero (native host processes)
