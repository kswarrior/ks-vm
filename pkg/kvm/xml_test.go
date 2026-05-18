package kvm

import (
	"strings"
	"testing"
)

func TestGenerateDomainXML(t *testing.T) {
	config := VMConfig{
		Name:     "test-vm",
		MemoryMB: 512,
		CPUs:     2,
		DiskPath: "/tmp/test.qcow2",
	}

	xml, err := GenerateDomainXML(config)
	if err != nil {
		t.Fatalf("Failed to generate XML: %v", err)
	}

	expectedParts := []string{
		"<name>test-vm</name>",
		"<memory unit=\"MiB\">512</memory>",
		"<vcpu>2</vcpu>",
		"<source file=\"/tmp/test.qcow2\"></source>",
		"bus=\"virtio\"",
	}

	for _, part := range expectedParts {
		if !strings.Contains(xml, part) {
			t.Errorf("XML missing expected part: %s", part)
		}
	}
}
