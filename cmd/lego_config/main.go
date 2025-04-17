//go:generate go run .
package main

import (
	"archive/zip"
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

const (
	repoURL   = "https://github.com/go-acme/lego/archive/refs/heads/master.zip"
	configDir = "internal/cert/config"
)

func main() {
	logger.Init("release")

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		logger.Error("Unable to get the current file")
		return
	}
	basePath := filepath.Join(filepath.Dir(file), "../../")

	zipFile, err := downloadAndExtract()
	if err != nil {
		logger.Errorf("Error downloading and extracting: %v\n", err)
		os.Exit(1)
	}

	if err := copyTomlFiles(zipFile, basePath); err != nil {
		logger.Errorf("Error copying TOML files: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Successfully updated provider config")
}

// downloadAndExtract downloads the lego repository and extracts it
func downloadAndExtract() (string, error) {
	// Download the file
	logger.Info("Downloading lego repository...")
	resp, err := http.Get(repoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.CreateTemp("", "lego-master.zip")
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

func copyTomlFiles(zipFile, basePath string) error {
	// Open the zip file
	logger.Info("Extracting files...")
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	// Extract files
	zfs := zipfs.New(&zipReader.Reader)
	afero.Walk(zfs, "./lego-master/providers", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
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
