package kvm

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type HostMetrics struct {
	CPUUsage  float64 `json:"cpu_usage"`
	MemTotal  uint64  `json:"mem_total"`
	MemUsed   uint64  `json:"mem_used"`
	DiskTotal uint64  `json:"disk_total"`
	DiskUsed  uint64  `json:"disk_used"`
	NetRecv   uint64  `json:"net_recv"`
	NetSent   uint64  `json:"net_sent"`
	Uptime    float64 `json:"uptime"`
	Kernel    string  `json:"kernel"`
}

var (
	prevIdle, prevTotal uint64
	cpuMu               sync.Mutex
)

func GetHostMetrics() (HostMetrics, error) {
	metrics := HostMetrics{}

	// CPU Usage
	idle, total, err := getCPUTime()
	if err == nil {
		cpuMu.Lock()
		if prevTotal > 0 {
			diffIdle := idle - prevIdle
			diffTotal := total - prevTotal
			if diffTotal > 0 {
				metrics.CPUUsage = (1.0 - float64(diffIdle)/float64(diffTotal)) * 100.0
			}
		}
		prevIdle, prevTotal = idle, total
		cpuMu.Unlock()
	}

	// RAM
	metrics.MemTotal, metrics.MemUsed, _ = getMemInfo()

	// Disk
	metrics.DiskTotal, metrics.DiskUsed, _ = getDiskUsage("/")

	// Network
	metrics.NetRecv, metrics.NetSent, _ = getNetStats()

	// Uptime
	metrics.Uptime, _ = getUptime()

	// Kernel
	if data, err := os.ReadFile("/proc/version"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 2 {
			metrics.Kernel = fields[2]
		}
	}

	return metrics, nil
}

func getCPUTime() (idle, total uint64, err error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("invalid /proc/stat format")
		}
		for i, field := range fields[1:] {
			val, _ := strconv.ParseUint(field, 10, 64)
			total += val
			if i == 3 { // idle is the 4th field (index 3 in fields[1:])
				idle = val
			}
		}
	}
	return idle, total, nil
}

func getMemInfo() (total, used uint64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var free, available uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		if fields[0] == "MemTotal:" {
			total = val
		} else if fields[0] == "MemAvailable:" {
			available = val
		} else if fields[0] == "MemFree:" {
			free = val
		}
	}
	// KB to MB
	if available > 0 {
		used = (total - available) / 1024
	} else {
		used = (total - free) / 1024
	}
	total /= 1024
	return total, used, nil
}

func getDiskUsage(path string) (total, used uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(path, &stat)
	if err != nil {
		return 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	used = (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
	return total / 1024 / 1024, used / 1024 / 1024, nil // MB
}

func getNetStats() (recv, sent uint64, err error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// Match common physical interface prefixes
		iface := strings.TrimSuffix(fields[0], ":")
		if strings.HasPrefix(iface, "eth") || strings.HasPrefix(iface, "en") || strings.HasPrefix(iface, "wl") {
			r, _ := strconv.ParseUint(fields[1], 10, 64)
			s, _ := strconv.ParseUint(fields[9], 10, 64)
			recv += r
			sent += s
		}
	}
	return recv, sent, nil
}

func getUptime() (float64, error) {
	file, err := os.Open("/proc/uptime")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var uptime float64
	fmt.Fscanf(file, "%f", &uptime)
	return uptime, nil
}
