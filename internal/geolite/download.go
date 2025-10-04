package geolite

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ulikunitz/xz"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/settings"
)

const (
	DownloadURL = "http://cloud.nginxui.com/geolite/GeoLite2-City.mmdb.xz"
)

type DownloadProgressWriter struct {
	io.Writer
	totalSize      int64
	currentSize    int64
	progressChan   chan<- float64
	lastReported   float64
	reportInterval float64 // Report only when progress changes by this amount
}

func (pw *DownloadProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	pw.currentSize += int64(n)
	progress := float64(pw.currentSize) / float64(pw.totalSize) * 100

	// Debounce: only send updates when progress changes by reportInterval or reaches 100%
	if progress-pw.lastReported >= pw.reportInterval || progress >= 100 {
		select {
		case pw.progressChan <- progress:
			pw.lastReported = progress
		default:
		}
	}
	return n, err
}

// GetDBPath returns the path to the GeoLite2 database file
func GetDBPath() string {
	confDir := filepath.Dir(settings.ConfPath)
	return filepath.Join(confDir, "GeoLite2-City.mmdb")
}

// GetDBXZPath returns the path to the compressed GeoLite2 database file
func GetDBXZPath() string {
	confDir := filepath.Dir(settings.ConfPath)
	return filepath.Join(confDir, "GeoLite2-City.mmdb.xz")
}

// DownloadGeoLiteDB downloads the GeoLite2 database
func DownloadGeoLiteDB(progressChan chan float64) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", DownloadURL, nil)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrDownloadFailed, err.Error())
	}

	resp, err := client.Do(req)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrDownloadFailed, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return cosy.WrapErrorWithParams(ErrDownloadFailed, fmt.Sprintf("status code: %d", resp.StatusCode))
	}

	totalSize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToGetFileSize, err.Error())
	}

	xzPath := GetDBXZPath()
	file, err := os.Create(xzPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToCreateFile, err.Error())
	}
	defer file.Close()

	progressWriter := &DownloadProgressWriter{
		Writer:         file,
		totalSize:      totalSize,
		progressChan:   progressChan,
		reportInterval: 1.0, // Report every 1% change
	}

	_, err = io.Copy(progressWriter, resp.Body)
	if err != nil {
		os.Remove(xzPath) // Clean up on error
		return cosy.WrapErrorWithParams(ErrFailedToSaveFile, err.Error())
	}

	return nil
}

// DecompressGeoLiteDB decompresses the .xz file to .mmdb
func DecompressGeoLiteDB(progressChan chan float64) error {
	xzPath := GetDBXZPath()
	dbPath := GetDBPath()

	// Open compressed file
	xzFile, err := os.Open(xzPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToOpenFile, err.Error())
	}
	defer xzFile.Close()

	// Get compressed file size
	fileInfo, err := xzFile.Stat()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToGetFileSize, err.Error())
	}
	compressedSize := fileInfo.Size()

	// Create XZ reader
	xzReader, err := xz.NewReader(xzFile)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToCreateXZReader, err.Error())
	}

	// Create output file
	outFile, err := os.Create(dbPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToCreateFile, err.Error())
	}
	defer outFile.Close()

	// Decompress with progress tracking
	buf := make([]byte, 64*1024) // 64KB buffer for better performance
	var decompressedSize int64
	var lastReportedProgress float64
	const reportInterval = 2.0 // Report every 2% change

	// Estimate: XZ typically compresses to 10-20% of original size
	// We'll use 15% (compression ratio ~6.67) as middle estimate
	const estimatedCompressionRatio = 6.67
	estimatedTotalSize := float64(compressedSize) * estimatedCompressionRatio

	for {
		n, readErr := xzReader.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				os.Remove(dbPath) // Clean up on error
				return cosy.WrapErrorWithParams(ErrFailedToWriteData, writeErr.Error())
			}
			decompressedSize += int64(n)

			// Calculate progress based on estimated total size
			progress := (float64(decompressedSize) / estimatedTotalSize) * 100
			if progress > 99 {
				progress = 99 // Cap at 99% until actually complete
			}

			// Debounce: only send updates when progress changes significantly
			if progress-lastReportedProgress >= reportInterval || readErr == io.EOF {
				select {
				case progressChan <- progress:
					lastReportedProgress = progress
				default:
				}
			}
		}
		if readErr == io.EOF {
			// Send 100% on completion
			select {
			case progressChan <- 100:
			default:
			}
			break
		}
		if readErr != nil {
			os.Remove(dbPath) // Clean up on error
			return cosy.WrapErrorWithParams(ErrFailedToReadData, readErr.Error())
		}
	}

	// Delete the .xz file after successful decompression
	if err := os.Remove(xzPath); err != nil {
		// Log but don't fail if we can't delete the compressed file
		return cosy.WrapErrorWithParams(ErrFailedToDeleteCompressed, err.Error())
	}

	return nil
}

// DBExists checks if the GeoLite2 database file exists
func DBExists() bool {
	_, err := os.Stat(GetDBPath())
	return err == nil
}
