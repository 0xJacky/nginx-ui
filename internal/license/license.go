package license

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/ulikunitz/xz"
)

//go:embed licenses.xz
var compressedLicenses []byte

type License struct {
	Name    string `json:"name"`
	License string `json:"license"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

type ComponentInfo struct {
	Backend  []License `json:"backend"`
	Frontend []License `json:"frontend"`
}

// GetLicenseInfo returns the license information for all components
func GetLicenseInfo() (*ComponentInfo, error) {
	if len(compressedLicenses) == 0 {
		return nil, fmt.Errorf("no license data available, run go generate to collect licenses")
	}

	// Decompress the xz data
	reader, err := xz.NewReader(bytes.NewReader(compressedLicenses))
	if err != nil {
		return nil, fmt.Errorf("failed to create xz reader: %v", err)
	}

	var decompressed bytes.Buffer
	_, err = decompressed.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress license data: %v", err)
	}

	// Parse JSON
	var info ComponentInfo
	if err := json.Unmarshal(decompressed.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("failed to parse license data: %v", err)
	}

	return &info, nil
}

// GetBackendLicenses returns only backend license information
func GetBackendLicenses() ([]License, error) {
	info, err := GetLicenseInfo()
	if err != nil {
		return nil, err
	}
	return info.Backend, nil
}

// GetFrontendLicenses returns only frontend license information
func GetFrontendLicenses() ([]License, error) {
	info, err := GetLicenseInfo()
	if err != nil {
		return nil, err
	}
	return info.Frontend, nil
}

// GetLicenseStats returns statistics about the licenses
func GetLicenseStats() (map[string]interface{}, error) {
	info, err := GetLicenseInfo()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	stats["total_backend"] = len(info.Backend)
	stats["total_frontend"] = len(info.Frontend)
	stats["total"] = len(info.Backend) + len(info.Frontend)

	// Count license types
	licenseCount := make(map[string]int)
	for _, license := range info.Backend {
		licenseCount[license.License]++
	}
	for _, license := range info.Frontend {
		licenseCount[license.License]++
	}

	stats["license_distribution"] = licenseCount
	return stats, nil
}
