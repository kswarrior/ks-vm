package container

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// PullAndUnpack pulls a Docker image and extracts it to the rootfs directory.
func PullAndUnpack(imageSource, destDir string) error {
	image := strings.TrimPrefix(imageSource, "docker://")
	parts := strings.Split(image, ":")
	repo := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	fmt.Printf("Pulling container image: %s:%s\n", repo, tag)

	// 1. Get Token
	token, err := getAuthToken(repo)
	if err != nil {
		return err
	}

	// 2. Get Manifest
	manifest, err := getManifest(repo, tag, token)
	if err != nil {
		return err
	}

	// 3. Extract RootFS
	rootfs := filepath.Join(destDir, "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return err
	}

	for _, layer := range manifest.Layers {
		fmt.Printf("Downloading and extracting layer: %s\n", layer.Digest)
		if err := downloadAndExtractLayer(repo, layer.Digest, token, rootfs); err != nil {
			fmt.Printf("Error extracting layer %s: %v\n", layer.Digest, err)
			return err
		}
	}

	return nil
}

type manifestV2 struct {
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

func getAuthToken(repo string) (string, error) {
	url := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	return data.Token, nil
}

func getManifest(repo, tag, token string) (*manifestV2, error) {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repo, tag)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// Support both single manifests and manifest lists (multi-arch)
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to get manifest: %s", resp.Status)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")

	if contentType == "application/vnd.docker.distribution.manifest.list.v2+json" {
		var list struct {
			Manifests []struct {
				Digest   string `json:"digest"`
				Platform struct {
					Architecture string `json:"architecture"`
					OS           string `json:"os"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		json.Unmarshal(body, &list)
		for _, m := range list.Manifests {
			if m.Platform.Architecture == "amd64" && m.Platform.OS == "linux" {
				return getManifest(repo, m.Digest, token)
			}
		}
		return nil, fmt.Errorf("no linux/amd64 manifest found in list")
	}

	var m manifestV2
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func downloadAndExtractLayer(repo, digest, token, dest string) error {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/blobs/%s", repo, digest)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize path to prevent Zip Slip (Path Traversal)
		relPath := filepath.Clean(header.Name)
		target := filepath.Join(dest, relPath)
		if !strings.HasPrefix(target, filepath.Clean(dest)) {
			continue // Skip files outside of destination
		}

		// Handle Docker whiteouts
		if strings.HasPrefix(filepath.Base(relPath), ".wh.") {
			if filepath.Base(relPath) == ".wh..wh.opq" {
				// Opaque whiteout: remove all entries in the directory
				dir := filepath.Dir(target)
				entries, _ := os.ReadDir(dir)
				for _, entry := range entries {
					if entry.Name() != ".wh..wh.opq" {
						os.RemoveAll(filepath.Join(dir, entry.Name()))
					}
				}
			} else {
				whiteoutFile := filepath.Join(filepath.Dir(target), strings.TrimPrefix(filepath.Base(relPath), ".wh."))
				os.RemoveAll(whiteoutFile)
			}
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			// Remove existing entry (file or dir) to handle layer type mismatches
			os.RemoveAll(target)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				continue
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.RemoveAll(target)
			os.Symlink(header.Linkname, target)
		case tar.TypeLink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.RemoveAll(target)
			oldTarget := filepath.Join(dest, header.Linkname)
			os.Link(oldTarget, target)
		}
	}
	return nil
}
