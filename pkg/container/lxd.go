package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

var LXDSockets = []string{
	"/var/snap/lxd/common/lxd/unix.socket",
	"/var/lib/lxd/unix.socket",
}

type LXDClient struct {
	client       *http.Client
	activeSocket string
}

func NewLXDClient() *LXDClient {
	return &LXDClient{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					// Fallback dialer will find the first available socket
					for _, s := range LXDSockets {
						conn, err := net.Dial("unix", s)
						if err == nil {
							return conn, nil
						}
					}
					return nil, fmt.Errorf("LXD socket not found in any standard location")
				},
			},
		},
	}
}

func (c *LXDClient) do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader bytes.Buffer
	if body != nil {
		json.NewEncoder(&bodyReader).Encode(body)
	}
	req, err := http.NewRequest(method, "http://lxd"+path, &bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "socket not found") || strings.Contains(err.Error(), "no such file or directory") {
			return nil, fmt.Errorf("LXD socket not found")
		}
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "permission denied") {
			return nil, fmt.Errorf("LXD service is inaccessible")
		}
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errData struct {
			Error     string `json:"error"`
			ErrorCode int    `json:"error_code"`
			ErrorType string `json:"error_type"`
		}
		json.NewDecoder(resp.Body).Decode(&errData)
		resp.Body.Close()
		errMsg := errData.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("LXD error: %s", errMsg)
	}

	return resp, nil
}

func (c *LXDClient) waitForOperation(opPath string) error {
	for i := 0; i < 30; i++ {
		resp, err := c.do("GET", opPath, nil)
		if err != nil {
			return err
		}
		var data struct {
			Metadata struct {
				Status string `json:"status"`
				Err    string `json:"err"`
			} `json:"metadata"`
		}
		json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()

		switch data.Metadata.Status {
		case "Success":
			return nil
		case "Failure":
			return fmt.Errorf("LXD operation failed: %s", data.Metadata.Err)
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for LXD operation")
}

func (c *LXDClient) ensureImage(image string) error {
	// 1. Check if image already exists locally by alias or fingerprint
	resp, err := c.do("GET", "/1.0/images/aliases/"+image, nil)
	if err == nil {
		resp.Body.Close()
		return nil
	}
	// Try fingerprint
	if len(image) == 64 {
		resp, err = c.do("GET", "/1.0/images/"+image, nil)
		if err == nil {
			resp.Body.Close()
			return nil
		}
	}

	// 2. Resolve image source
	sourceURL := "https://images.linuxcontainers.org"
	imgName := image
	sourceType := "simplestreams"

	if strings.Contains(image, ":") {
		parts := strings.Split(image, ":")
		if parts[0] == "ubuntu" {
			// Official Ubuntu images use specific streams
			sourceURL = "https://cloud-images.ubuntu.com/releases"
			imgName = parts[1]
		}
	}

	// 3. Pull image
	fmt.Printf("Image %s not found locally, pulling from %s (%s)...\n", image, sourceURL, imgName)

	body := map[string]interface{}{
		"source": map[string]string{
			"type":   sourceType,
			"url":    sourceURL,
			"alias":  imgName, // simplestreams often expects 'alias' instead of 'name' in some versions
		},
		"aliases": []map[string]interface{}{
			{"name": image},
		},
		"public": false,
	}

	// Double check: if it's linuxcontainers.org, it might need 'name' instead
	if strings.Contains(sourceURL, "linuxcontainers.org") {
		body["source"].(map[string]string)["name"] = imgName
	}
	resp, err = c.do("POST", "/1.0/images", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var opData struct {
		Operation string `json:"operation"`
	}
	json.NewDecoder(resp.Body).Decode(&opData)
	return c.waitForOperation(opData.Operation)
}

func (c *LXDClient) getStoragePool() string {
	resp, err := c.do("GET", "/1.0/storage-pools", nil)
	if err == nil {
		defer resp.Body.Close()
		var data struct {
			Metadata []string `json:"metadata"`
		}
		json.NewDecoder(resp.Body).Decode(&data)
		if len(data.Metadata) > 0 {
			parts := strings.Split(data.Metadata[0], "/")
			return parts[len(parts)-1]
		}
	}
	return "default"
}

func (c *LXDClient) CreateContainer(name string, cpu uint, ramMB uint, diskGB uint, image string) error {
	if err := c.ensureImage(image); err != nil {
		return fmt.Errorf("failed to ensure image: %v", err)
	}

	// Determine if we're using a fingerprint or an alias
	source := map[string]string{
		"type": "image",
	}
	if len(image) == 64 && !strings.Contains(image, ":") && !strings.Contains(image, ".") {
		source["fingerprint"] = image
	} else {
		source["alias"] = image
	}

	pool := c.getStoragePool()

	config := map[string]interface{}{
		"name":   name,
		"source": source,
		"config": map[string]string{
			"limits.cpu":    fmt.Sprintf("%d", cpu),
			"limits.memory": fmt.Sprintf("%dMB", ramMB),
		},
		"devices": map[string]interface{}{
			"root": map[string]string{
				"path": "/",
				"pool": pool,
				"type": "disk",
				"size": fmt.Sprintf("%dGB", diskGB),
			},
		},
	}

	resp, err := c.do("POST", "/1.0/instances", config)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		var errData struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errData)
		return fmt.Errorf("LXD error (%d): %s", resp.StatusCode, errData.Error)
	}

	var opData struct {
		Operation string `json:"operation"`
	}
	json.NewDecoder(resp.Body).Decode(&opData)
	return c.waitForOperation(opData.Operation)
}

func (c *LXDClient) ControlContainer(name, action string) error {
	var lxdAction string
	switch action {
	case "start":
		lxdAction = "start"
	case "stop":
		lxdAction = "stop"
	case "restart":
		lxdAction = "restart"
	case "delete":
		// LXD requires instance to be stopped before deletion
		c.do("PUT", "/1.0/instances/"+name+"/state", map[string]string{"action": "stop", "force": "true"})

		resp, err := c.do("DELETE", "/1.0/instances/"+name, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var opData struct {
			Operation string `json:"operation"`
		}
		json.NewDecoder(resp.Body).Decode(&opData)
		return c.waitForOperation(opData.Operation)
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	reqBody := map[string]string{"action": lxdAction}
	resp, err := c.do("PUT", "/1.0/instances/"+name+"/state", reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var opData struct {
		Operation string `json:"operation"`
	}
	json.NewDecoder(resp.Body).Decode(&opData)
	return c.waitForOperation(opData.Operation)
}

func (c *LXDClient) EditContainerResources(name string, cpu uint, ramMB uint) error {
	config := map[string]interface{}{
		"config": map[string]string{
			"limits.cpu":    fmt.Sprintf("%d", cpu),
			"limits.memory": fmt.Sprintf("%dMB", ramMB),
		},
	}
	resp, err := c.do("PATCH", "/1.0/instances/"+name, config)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type ContainerMetrics struct {
	CPUUsage    float64  `json:"cpu_usage"`
	MemoryUsed  uint64   `json:"memory_used"`
	MemoryTotal uint64   `json:"memory_total"`
	DiskUsed    uint64   `json:"disk_used"`
	DiskTotal   uint64   `json:"disk_total"`
	Status      string   `json:"status"`
	Logs        string   `json:"logs"`
	IPs         []string `json:"ips"`
}

func (c *LXDClient) GetContainerMetrics(name string) (*ContainerMetrics, error) {
	resp, err := c.do("GET", "/1.0/instances/"+name+"/state", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Metadata struct {
			Status string `json:"status"`
			Memory struct {
				Usage uint64 `json:"usage"`
			} `json:"memory"`
			CPU struct {
				Usage uint64 `json:"usage"`
			} `json:"cpu"`
			Disk map[string]struct {
				Usage uint64 `json:"usage"`
			} `json:"disk"`
			Network map[string]struct {
				Addresses []struct {
					Address string `json:"address"`
					Family  string `json:"family"`
					Scope   string `json:"scope"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"metadata"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	metrics := &ContainerMetrics{
		Status:     strings.ToLower(data.Metadata.Status),
		MemoryUsed: data.Metadata.Memory.Usage / 1024 / 1024,
	}

	for _, disk := range data.Metadata.Disk {
		metrics.DiskUsed = disk.Usage / 1024 / 1024 / 1024
		break // Usually just one root disk
	}

	for _, net := range data.Metadata.Network {
		for _, addr := range net.Addresses {
			if addr.Family == "inet" && addr.Scope == "global" {
				metrics.IPs = append(metrics.IPs, addr.Address)
			}
		}
	}

	// Fetch instance config for limits
	resp2, _ := c.do("GET", "/1.0/instances/"+name, nil)
	if resp2 != nil {
		defer resp2.Body.Close()
		var instData struct {
			Metadata struct {
				Config  map[string]string `json:"config"`
				Devices map[string]struct {
					Size string `json:"size"`
				} `json:"devices"`
			} `json:"metadata"`
		}
		json.NewDecoder(resp2.Body).Decode(&instData)
		if mem, ok := instData.Metadata.Config["limits.memory"]; ok {
			fmt.Sscanf(mem, "%dMB", &metrics.MemoryTotal)
		}
		if root, ok := instData.Metadata.Devices["root"]; ok {
			fmt.Sscanf(root.Size, "%dGB", &metrics.DiskTotal)
		}
	}

	return metrics, nil
}

func (c *LXDClient) ListContainers() ([]string, error) {
	resp, err := c.do("GET", "/1.0/instances", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Metadata []string `json:"metadata"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	var names []string
	for _, url := range data.Metadata {
		parts := strings.Split(url, "/")
		names = append(names, parts[len(parts)-1])
	}
	return names, nil
}

func (c *LXDClient) StreamContainerSSH(name string) (string, error) {
	// This normally involves a websocket upgrade. In this context, we return a "best effort" exec path
	// or instructions for the gateway to bridge the socket.
	// For "KS VM", we'll return a command that can be used by nsenter or lxc exec.
	return fmt.Sprintf("lxc exec %s -- /bin/bash", name), nil
}
