package kvm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/progressbar/v3"
)

const (
	ImagesDir = "/var/lib/ksvm/images"
)

// ImageInfo contains metadata about a registered base image.
type ImageInfo struct {
	Name    string
	Size    int64
	AddedAt time.Time
	Path    string
}

// DownloadImage downloads an image from a URL with a progress bar.
func DownloadImage(name, url string) (string, error) {
	if err := os.MkdirAll(ImagesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create images directory: %v", err)
	}

	destPath := filepath.Join(ImagesDir, name+".qcow2")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download: status %s", resp.Status)
	}

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading "+name,
	)

	_, err = io.Copy(io.MultiWriter(f, bar), resp.Body)
	if err != nil {
		return "", err
	}

	return destPath, nil
}

// ListImages returns metadata for all registered base images.
func ListImages() ([]ImageInfo, error) {
	if _, err := os.Stat(ImagesDir); os.IsNotExist(err) {
		return []ImageInfo{}, nil
	}

	entries, err := os.ReadDir(ImagesDir)
	if err != nil {
		return nil, err
	}

	var images []ImageInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		images = append(images, ImageInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			AddedAt: info.ModTime(),
			Path:    filepath.Join(ImagesDir, entry.Name()),
		})
	}

	return images, nil
}

// RemoveImage deletes a base image from the pool.
func RemoveImage(name string) error {
	path := filepath.Join(ImagesDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(ImagesDir, name+".qcow2")
	}

	return os.Remove(path)
}

// RegisterLocalImage copies or symlinks a local image to the images directory.
func RegisterLocalImage(name, localPath string) (string, error) {
	if err := os.MkdirAll(ImagesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create images directory: %v", err)
	}

	destPath := filepath.Join(ImagesDir, name+".qcow2")

	if filepath.Clean(localPath) == filepath.Clean(destPath) {
		return destPath, nil
	}

	src, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return "", err
	}

	return destPath, nil
}
