package backup

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// Directory and file names
const (
	BackupDirPrefix = "nginx-ui-backup-"
	NginxUIDir      = "nginx-ui"
	NginxDir        = "nginx"
	HashInfoFile    = "hash_info.txt"
	NginxUIZipName  = "nginx-ui.zip"
	NginxZipName    = "nginx.zip"
)

// BackupResult contains the results of a backup operation
type BackupResult struct {
	BackupContent []byte `json:"-"`       // Backup content as byte array
	BackupName    string `json:"name"`    // Backup file name
	AESKey        string `json:"aes_key"` // Base64 encoded AES key
	AESIv         string `json:"aes_iv"`  // Base64 encoded AES IV
}

// HashInfo contains hash information for verification
type HashInfo struct {
	NginxUIHash string `json:"nginx_ui_hash"`
	NginxHash   string `json:"nginx_hash"`
	Timestamp   string `json:"timestamp"`
	Version     string `json:"version"`
}

// Backup creates a backup of nginx-ui configuration and database files,
// and nginx configuration directory, compressed into an encrypted archive
func Backup() (BackupResult, error) {
	// Generate timestamps for filenames
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("backup-%s.zip", timestamp)

	// Generate AES key and IV
	key, err := GenerateAESKey()
	if err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrGenerateAESKey, err.Error())
	}

	iv, err := GenerateIV()
	if err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrGenerateIV, err.Error())
	}

	// Create temporary directory for files to be archived
	tempDir, err := os.MkdirTemp("", "nginx-ui-backup-*")
	if err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateTempDir, err.Error())
	}
	defer os.RemoveAll(tempDir)

	// Create directories in temp
	nginxUITempDir := filepath.Join(tempDir, NginxUIDir)
	nginxTempDir := filepath.Join(tempDir, NginxDir)
	if err := os.MkdirAll(nginxUITempDir, 0755); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateTempSubDir, err.Error())
	}
	if err := os.MkdirAll(nginxTempDir, 0755); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateTempSubDir, err.Error())
	}

	// Backup nginx-ui config and database to a directory
	if err := backupNginxUIFiles(nginxUITempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrBackupNginxUI, err.Error())
	}

	// Backup nginx configs to a directory
	if err := backupNginxFiles(nginxTempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrBackupNginx, err.Error())
	}

	// Create individual zip files for nginx-ui and nginx directories
	nginxUIZipPath := filepath.Join(tempDir, NginxUIZipName)
	nginxZipPath := filepath.Join(tempDir, NginxZipName)

	// Create zip archives for each directory
	if err := createZipArchive(nginxUIZipPath, nginxUITempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	if err := createZipArchive(nginxZipPath, nginxTempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	// Calculate hashes for the zip files
	nginxUIHash, err := calculateFileHash(nginxUIZipPath)
	if err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCalculateHash, err.Error())
	}

	nginxHash, err := calculateFileHash(nginxZipPath)
	if err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCalculateHash, err.Error())
	}

	// Get current version information
	versionInfo := version.GetVersionInfo()

	// Create hash info file
	hashInfo := HashInfo{
		NginxUIHash: nginxUIHash,
		NginxHash:   nginxHash,
		Timestamp:   timestamp,
		Version:     versionInfo.Version,
	}

	// Write hash info to file
	hashInfoPath := filepath.Join(tempDir, HashInfoFile)
	if err := writeHashInfoFile(hashInfoPath, hashInfo); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateHashFile, err.Error())
	}

	// Encrypt the individual files
	if err := encryptFile(hashInfoPath, key, iv); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrEncryptFile, HashInfoFile)
	}

	if err := encryptFile(nginxUIZipPath, key, iv); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrEncryptNginxUIDir, err.Error())
	}

	if err := encryptFile(nginxZipPath, key, iv); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrEncryptNginxDir, err.Error())
	}

	// Remove the original directories to avoid duplicating them in the final archive
	if err := os.RemoveAll(nginxUITempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCleanupTempDir, err.Error())
	}
	if err := os.RemoveAll(nginxTempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCleanupTempDir, err.Error())
	}

	// Create final zip file to memory buffer
	var buffer bytes.Buffer
	if err := createZipArchiveToBuffer(&buffer, tempDir); err != nil {
		return BackupResult{}, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	// Convert AES key and IV to base64 encoded strings
	keyBase64 := base64.StdEncoding.EncodeToString(key)
	ivBase64 := base64.StdEncoding.EncodeToString(iv)

	// Return result
	result := BackupResult{
		BackupContent: buffer.Bytes(),
		BackupName:    backupName,
		AESKey:        keyBase64,
		AESIv:         ivBase64,
	}

	logger.Infof("Backup created successfully: %s", backupName)
	return result, nil
}
