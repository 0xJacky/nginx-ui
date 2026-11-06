package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Define base directory for map components
	baseDir := "app/src/views/nginx_log/dashboard/components"
	
	// Map configurations
	maps := []struct {
		name     string
		url      string
		jsonFile string
	}{
		{
			name:     "WorldMapChart",
			url:      "https://cdn.jsdelivr.net/npm/echarts/map/json/world.json",
			jsonFile: "world.json",
		},
		{
			name:     "ChinaMapChart", 
			url:      "https://cdn.jsdelivr.net/npm/echarts/map/json/china.json",
			jsonFile: "china.json",
		},
	}

	for _, mapConfig := range maps {
		// Create directory for the map component
		mapDir := filepath.Join(baseDir, mapConfig.name)
		if err := os.MkdirAll(mapDir, 0755); err != nil {
			fmt.Printf("Failed to create directory %s: %v\n", mapDir, err)
			continue
		}

		// Download JSON data
		jsonPath := filepath.Join(mapDir, mapConfig.jsonFile)
		if err := downloadFile(mapConfig.url, jsonPath); err != nil {
			fmt.Printf("Failed to download %s: %v\n", mapConfig.url, err)
			continue
		}
		fmt.Printf("Downloaded %s to %s\n", mapConfig.url, jsonPath)
	}

	fmt.Println("Map generator completed successfully!")
}

// downloadFile downloads a file from URL and saves it to the specified path
func downloadFile(url, filepath string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// moveFile moves a file from src to dst
func moveFile(src, dst string) error {
	// Check if source file exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", src)
	}

	// Attempt to rename first (fastest if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If rename fails, copy and delete
	return copyAndDelete(src, dst)
}

// copyAndDelete copies a file and then deletes the original
func copyAndDelete(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy the content
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Sync to ensure all data is written
	if err := dstFile.Sync(); err != nil {
		return err
	}

	// Remove source file
	return os.Remove(src)
}