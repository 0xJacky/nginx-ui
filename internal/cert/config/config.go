package config

import (
	"archive/tar"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"

	"github.com/ulikunitz/xz"
)

//go:embed config.tar.xz
var compressedData []byte

var decompressedFS map[string][]byte

func init() {
	var err error
	decompressedFS, err = decompressConfigs()
	if err != nil {
		log.Fatalf("Failed to decompress config files: %v", err)
	}
}

// GetConfig returns the content of a specific TOML config file
func GetConfig(filename string) ([]byte, error) {
	data, exists := decompressedFS[filename]
	if !exists {
		return nil, fmt.Errorf("config file %s not found", filename)
	}
	
	return data, nil
}

// ListConfigs returns a list of available config filenames
func ListConfigs() ([]string, error) {
	var filenames []string
	for filename := range decompressedFS {
		filenames = append(filenames, filename)
	}
	
	return filenames, nil
}

// decompressConfigs decompresses the embedded XZ archive and returns the files as a map
func decompressConfigs() (map[string][]byte, error) {
	// Decompress XZ data
	xzReader, err := xz.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create xz reader: %w", err)
	}
	
	// Read decompressed tar data
	tarReader := tar.NewReader(xzReader)
	
	files := make(map[string][]byte)
	
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}
		
		if header.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", header.Name, err)
			}
			
			files[header.Name] = data
		}
	}
	
	return files, nil
}
