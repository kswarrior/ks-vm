package container

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"encoding/json"
	"time"
)

type DockerClient struct {
	client *http.Client
}

func NewDockerClient() *DockerClient {
	return &DockerClient{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", "/var/run/docker.sock")
				},
			},
		},
	}
}

func (c *DockerClient) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, "http://localhost"+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.client.Do(req)
}

func (c *DockerClient) PullImage(image string) error {
	resp, err := c.do("POST", "/images/create?fromImage="+image, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Docker pull returns a stream of JSON messages
	dec := json.NewDecoder(resp.Body)
	for {
		var m map[string]interface{}
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (c *DockerClient) CreateContainer(name, image string, cpus uint, memoryMB uint) (string, error) {
	config := map[string]interface{}{
		"Image": image,
		"HostConfig": map[string]interface{}{
			"Memory":     int64(memoryMB) * 1024 * 1024,
			"NanoCpus":   int64(cpus) * 1e9,
			"NetworkMode": "bridge",
		},
	}

	body, _ := json.Marshal(config)
	resp, err := c.do("POST", "/containers/create?name="+name, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errData struct{ Message string }
		json.NewDecoder(resp.Body).Decode(&errData)
		return "", fmt.Errorf("docker error: %s", errData.Message)
	}

	var data struct{ Id string }
	json.NewDecoder(resp.Body).Decode(&data)
	return data.Id, nil
}

func (c *DockerClient) StartContainer(name string) error {
	resp, err := c.do("POST", "/containers/"+name+"/start", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to start container: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) StopContainer(name string) error {
	resp, err := c.do("POST", "/containers/"+name+"/stop", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *DockerClient) RemoveContainer(name string) error {
	resp, err := c.do("DELETE", "/containers/"+name+"?force=true", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *DockerClient) ListContainers() ([]string, error) {
	resp, err := c.do("GET", "/containers/json?all=true", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var containers []struct {
		Names []string `json:"Names"`
	}
	json.NewDecoder(resp.Body).Decode(&containers)

	var names []string
	for _, cont := range containers {
		for _, n := range cont.Names {
			names = append(names, strings.TrimPrefix(n, "/"))
		}
	}
	return names, nil
}

type DockerMetrics struct {
	Status string
	MemoryUsed uint64
	MemoryTotal uint64
	IPs []string
	Uptime int64
}

func (c *DockerClient) GetContainerMetrics(name string) (*DockerMetrics, error) {
	resp, err := c.do("GET", "/containers/"+name+"/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		State struct {
			Status string
			StartedAt string
		}
		NetworkSettings struct {
			IPAddress string
			Networks map[string]struct {
				IPAddress string
			}
		}
		HostConfig struct {
			Memory uint64
		}
	}
	json.NewDecoder(resp.Body).Decode(&data)

	ips := []string{}
	if data.NetworkSettings.IPAddress != "" {
		ips = append(ips, data.NetworkSettings.IPAddress)
	}
	for _, net := range data.NetworkSettings.Networks {
		if net.IPAddress != "" && net.IPAddress != data.NetworkSettings.IPAddress {
			ips = append(ips, net.IPAddress)
		}
	}

	uptime := int64(0)
	if t, err := time.Parse(time.RFC3339Nano, data.State.StartedAt); err == nil {
		uptime = int64(time.Since(t).Seconds())
	}

	return &DockerMetrics{
		Status: data.State.Status,
		MemoryTotal: data.HostConfig.Memory / 1024 / 1024,
		IPs: ips,
		Uptime: uptime,
	}, nil
}
