//go:generate go run .
package main

import (
	"archive/zip"
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

	logger.Info("Successfully updated provider config")
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
