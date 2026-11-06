package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxMind GeoLite2 databases (free)
	cityDBURL     = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=YOUR_LICENSE_KEY&suffix=tar.gz"
	countryDBURL  = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=YOUR_LICENSE_KEY&suffix=tar.gz"
	outputDir     = "internal/geolite"
	cityDBName    = "GeoLite2-City.mmdb"
	countryDBName = "GeoLite2-Country.mmdb"
)

func main() {
	fmt.Println("MaxMind GeoLite2 Database Downloader")
	fmt.Println("====================================")
	fmt.Println()
	fmt.Println("Note: This script requires a MaxMind license key.")
	fmt.Println("You can get a free license key by signing up at:")
	fmt.Println("https://www.maxmind.com/en/geolite2/signup")
	fmt.Println()
	fmt.Println("Alternative: Download manually and place the .mmdb files in internal/geolite/")
	fmt.Println()
	
	// Check if license key is provided via environment variable
	licenseKey := os.Getenv("MAXMIND_LICENSE_KEY")
	if licenseKey == "" {
		fmt.Println("MAXMIND_LICENSE_KEY environment variable not set.")
		fmt.Println("Usage: MAXMIND_LICENSE_KEY=your_key go run cmd/geolite_downloader/main.go")
		fmt.Println()
		fmt.Println("For manual download:")
		fmt.Printf("1. Download GeoLite2-City.mmdb to %s/\n", outputDir)
		fmt.Printf("2. Download GeoLite2-Country.mmdb to %s/\n", outputDir)
		return
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		return
	}

	// Download databases
	fmt.Println("Downloading GeoLite2 databases...")
	
	if err := downloadDatabase(strings.Replace(countryDBURL, "YOUR_LICENSE_KEY", licenseKey, 1), 
		filepath.Join(outputDir, countryDBName)); err != nil {
		fmt.Printf("Failed to download Country database: %v\n", err)
	} else {
		fmt.Printf("✓ Downloaded %s\n", countryDBName)
	}

	if err := downloadDatabase(strings.Replace(cityDBURL, "YOUR_LICENSE_KEY", licenseKey, 1), 
		filepath.Join(outputDir, cityDBName)); err != nil {
		fmt.Printf("Failed to download City database: %v\n", err)
	} else {
		fmt.Printf("✓ Downloaded %s\n", cityDBName)
	}

	fmt.Println()
	fmt.Println("Download completed!")
}

func downloadDatabase(url, outputPath string) error {
	// Download the tar.gz file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create a gzip reader
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract the .mmdb file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for .mmdb file
		if strings.HasSuffix(header.Name, ".mmdb") {
			// Create output file
			outFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer outFile.Close()

			// Copy the file content
			_, err = io.Copy(outFile, tarReader)
			if err != nil {
				return fmt.Errorf("failed to extract file: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("no .mmdb file found in archive")
}