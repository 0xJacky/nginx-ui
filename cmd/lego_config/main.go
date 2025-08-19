//go:generate go run .
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"runtime"

	"github.com/spf13/afero"
	"github.com/spf13/afero/zipfs"
	"github.com/uozi-tech/cosy/logger"
	"github.com/ulikunitz/xz"
)

// GitHubRelease represents the structure of GitHub's release API response
type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

const (
	githubAPIURL = "https://cloud.nginxui.com/https://api.github.com/repos/go-acme/lego/releases/latest"
	configDir    = "internal/cert/config"
)

func main() {
	logger.Init("release")

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		logger.Error("Unable to get the current file")
		return
	}
	basePath := filepath.Join(filepath.Dir(file), "../../")

	// Get the latest release tag
	tag, err := getLatestReleaseTag()
	if err != nil {
		logger.Errorf("Error getting latest release tag: %v\n", err)
		os.Exit(1)
	}
	logger.Infof("Latest release tag: %s", tag)

	zipFile, err := downloadAndExtract(tag)
	if err != nil {
		logger.Errorf("Error downloading and extracting: %v\n", err)
		os.Exit(1)
	}

	if err := copyTomlFiles(zipFile, basePath, tag); err != nil {
		logger.Errorf("Error copying TOML files: %v\n", err)
		os.Exit(1)
	}

	if err := compressConfigs(basePath); err != nil {
		logger.Errorf("Error compressing configs: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Successfully updated and compressed provider config")
}

// getLatestReleaseTag fetches the latest release tag from GitHub API
func getLatestReleaseTag() (string, error) {
	logger.Info("Fetching latest release tag...")

	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return "", err
	}

	// Add User-Agent header to avoid GitHub API limitations
	req.Header.Set("User-Agent", "NGINX-UI-LegoConfigure")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status from GitHub API: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag name found in the latest release")
	}

	return release.TagName, nil
}

// downloadAndExtract downloads the lego repository for a specific tag and extracts it
func downloadAndExtract(tag string) (string, error) {
	downloadURL := fmt.Sprintf("https://cloud.nginxui.com/https://github.com/go-acme/lego/archive/refs/tags/%s.zip", tag)

	// Download the file
	logger.Infof("Downloading lego repository for tag %s...", tag)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.CreateTemp("", "lego-"+tag+".zip")
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return out.Name(), nil
}

func copyTomlFiles(zipFile, basePath, tag string) error {
	// Open the zip file
	logger.Info("Extracting files...")
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	// Extract files
	tag = strings.TrimPrefix(tag, "v")
	zfs := zipfs.New(&zipReader.Reader)
	afero.Walk(zfs, "./lego-"+tag+"/providers", func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".toml") {
			return nil
		}
		if err != nil {
			return err
		}
		data, err := afero.ReadFile(zfs, path)
		if err != nil {
			return err
		}
		// Write to the destination file
		destPath := filepath.Join(basePath, configDir, info.Name())
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
		logger.Infof("Copied: %s", info.Name())
		return nil
	})

	// Clean up zip file
	return os.Remove(zipFile)
}

// compressConfigs compresses all TOML files into a single XZ archive
func compressConfigs(basePath string) error {
	logger.Info("Compressing config files...")
	
	configDir := filepath.Join(basePath, "internal/cert/config")
	
	// Create buffer for tar data
	var tarBuffer bytes.Buffer
	tarWriter := tar.NewWriter(&tarBuffer)
	
	// Walk through TOML files and add to tar
	err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !strings.HasSuffix(info.Name(), ".toml") {
			return nil
		}
		
		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		// Create tar header
		header := &tar.Header{
			Name: info.Name(),
			Mode: 0644,
			Size: int64(len(data)),
		}
		
		// Write header and data to tar
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		
		if _, err := tarWriter.Write(data); err != nil {
			return err
		}
		
		logger.Infof("Added to archive: %s", info.Name())
		return nil
	})
	
	if err != nil {
		return err
	}
	
	if err := tarWriter.Close(); err != nil {
		return err
	}
	
	// Compress with XZ
	var compressedBuffer bytes.Buffer
	xzWriter, err := xz.NewWriter(&compressedBuffer)
	if err != nil {
		return err
	}
	
	if _, err := xzWriter.Write(tarBuffer.Bytes()); err != nil {
		return err
	}
	
	if err := xzWriter.Close(); err != nil {
		return err
	}
	
	// Write compressed data to config.tar.xz
	compressedPath := filepath.Join(configDir, "config.tar.xz")
	if err := os.WriteFile(compressedPath, compressedBuffer.Bytes(), 0644); err != nil {
		return err
	}
	
	// Remove individual TOML files
	err = filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if strings.HasSuffix(info.Name(), ".toml") {
			if err := os.Remove(path); err != nil {
				return err
			}
			logger.Infof("Removed: %s", info.Name())
		}
		
		return nil
	})
	
	if err != nil {
		return err
	}
	
	logger.Infof("Compressed config saved to: %s", compressedPath)
	return nil
}
