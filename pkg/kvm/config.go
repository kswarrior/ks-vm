package kvm

import (
	"os"
)

var BaseDir = getBaseDir()
var ImagesDir = BaseDir + "/images"

func getBaseDir() string {
	d := os.Getenv("KSVM_BASE_DIR")
	if d == "" {
		return "/var/lib/ksvm"
	}
	return d
}
