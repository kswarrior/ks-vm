package kvm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateCloudInitISO creates a NoCloud ISO containing user-data and meta-data.
func (m *Manager) GenerateCloudInitISO(rootDir, user, password string) (string, error) {
	userData := fmt.Sprintf(`#cloud-config
users:
  - name: %s
    groups: sudo
    shell: /bin/bash
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    lock_passwd: false
    passwd: %s
chpasswd:
  list: |
    %s:%s
  expire: False
ssh_pwauth: True
runcmd:
  - echo "%s:%s" | chpasswd
`, user, password, user, password, user, password)
	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", filepath.Base(rootDir), filepath.Base(rootDir))

	configDir := filepath.Join(rootDir, "cloud-init")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "user-data"), []byte(userData), 0644)
	os.WriteFile(filepath.Join(configDir, "meta-data"), []byte(metaData), 0644)

	isoPath := filepath.Join(rootDir, "cloud-init.iso")

	// Search for an available ISO tool
	var tool string
	candidates := []string{"xorrisofs", "genisoimage", "mkisofs", "xorriso"}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			tool = path
			break
		}
	}

	if tool == "" {
		return "", fmt.Errorf("no ISO generation tool found (xorrisofs, genisoimage, mkisofs). Please install 'xorriso' or 'genisoimage'")
	}

	var cmd *exec.Cmd
	if filepath.Base(tool) == "xorriso" {
		cmd = exec.Command(tool, "-as", "mkisofs", "-output", isoPath, "-volid", "cidata", "-joliet", "-rock", configDir)
	} else {
		cmd = exec.Command(tool, "-output", isoPath, "-volid", "cidata", "-joliet", "-rock", configDir)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create cloud-init ISO using %s: %v, output: %s", tool, err, string(output))
	}

	return isoPath, nil
}
