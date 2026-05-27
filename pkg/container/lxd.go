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
			return nil, fmt.Errorf("LXD socket not found. Please ensure LXD is installed and initialized (run 'lxd init').")
		}
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "permission denied") {
			return nil, fmt.Errorf("LXD service is inaccessible (check permissions or if service is running).")
		}
		return nil, err
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
	// Check if image exists
	resp, err := c.do("GET", "/1.0/images/aliases/"+image, nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Pull from Ubuntu simplestreams if not found
	fmt.Printf("Image %s not found, pulling from Ubuntu streams...\n", image)
	body := map[string]interface{}{
		"source": map[string]string{
			"type": "simplestreams",
			"url":  "https://images.linuxcontainers.org",
			"name": image,
		},
		"aliases": []map[string]string{{"name": image}},
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

func (c *LXDClient) CreateContainer(name string, cpu uint, ramMB uint, diskGB uint, image string) error {
	if err := c.ensureImage(image); err != nil {
		return fmt.Errorf("failed to ensure image: %v", err)
	}

	config := map[string]interface{}{
		"name": name,
		"source": map[string]string{
			"type":  "image",
			"alias": image,
		},
		"config": map[string]string{
			"limits.cpu":    fmt.Sprintf("%d", cpu),
			"limits.memory": fmt.Sprintf("%dMB", ramMB),
		},
		"devices": map[string]interface{}{
			"root": map[string]string{
				"path": "/",
				"pool": "default",
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
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryTotal uint64  `json:"memory_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	Status      string  `json:"status"`
	Logs        string  `json:"logs"`
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
		} `json:"metadata"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	metrics := &ContainerMetrics{
		Status:     strings.ToLower(data.Metadata.Status),
		MemoryUsed: data.Metadata.Memory.Usage / 1024 / 1024,
	}

	// Fetch instance config for limits
	resp2, _ := c.do("GET", "/1.0/instances/"+name, nil)
	if resp2 != nil {
		defer resp2.Body.Close()
		var instData struct {
			Metadata struct {
				Config map[string]string `json:"config"`
			} `json:"metadata"`
		}
		json.NewDecoder(resp2.Body).Decode(&instData)
		fmt.Sscanf(instData.Metadata.Config["limits.memory"], "%dMB", &metrics.MemoryTotal)
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
