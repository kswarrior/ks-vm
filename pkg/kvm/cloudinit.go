package kvm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateCloudInitISO creates a NoCloud ISO containing user-data and meta-data.
func (m *Manager) GenerateCloudInitISO(rootDir, user, password string) (string, error) {
	userData := fmt.Sprintf("#cloud-config\nuser: %s\npassword: %s\nchpasswd: { expire: False }\nssh_pwauth: True\n", user, password)
	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", filepath.Base(rootDir), filepath.Base(rootDir))

	configDir := filepath.Join(rootDir, "cloud-init")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "user-data"), []byte(userData), 0644)
	os.WriteFile(filepath.Join(configDir, "meta-data"), []byte(metaData), 0644)

	isoPath := filepath.Join(rootDir, "cloud-init.iso")

	// Use xorrisofs as a replacement for genisoimage
	cmd := exec.Command("xorrisofs", "-output", isoPath, "-volid", "cidata", "-joliet", "-rock", configDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create cloud-init ISO: %v, output: %s", err, string(output))
	}

	return isoPath, nil
}
