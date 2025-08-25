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

// Constants for backup directory and file naming conventions
const (
	NginxUIDir     = "nginx-ui"      // Directory name for Nginx UI files in backup
	NginxDir       = "nginx"         // Directory name for Nginx config files in backup
	HashInfoFile   = "hash_info.txt" // Filename for hash verification information
	NginxUIZipName = "nginx-ui.zip"  // Filename for Nginx UI archive within backup
	NginxZipName   = "nginx.zip"     // Filename for Nginx config archive within backup
)

// Result contains the complete results of a backup operation.
// This structure encapsulates all data needed to restore or verify a backup.
type Result struct {
	BackupContent []byte `json:"-"`       // Encrypted backup content as byte array (excluded from JSON)
	BackupName    string `json:"name"`    // Generated backup filename with timestamp
	AESKey        string `json:"aes_key"` // Base64 encoded AES encryption key
	AESIv         string `json:"aes_iv"`  // Base64 encoded AES initialization vector
}

// HashInfo contains cryptographic hash information for backup verification.
// This structure ensures backup integrity and provides metadata for restoration.
type HashInfo struct {
	NginxUIHash string `json:"nginx_ui_hash"` // SHA-256 hash of Nginx UI files archive
	NginxHash   string `json:"nginx_hash"`    // SHA-256 hash of Nginx config files archive
	Timestamp   string `json:"timestamp"`     // Backup creation timestamp
	Version     string `json:"version"`       // Nginx UI version at backup time
}

// Backup creates a comprehensive backup of nginx-ui configuration, database files,
// and nginx configuration directory. The backup is compressed and encrypted for security.
//
// The backup process includes:
//  1. Creating temporary directories for staging files
//  2. Copying Nginx UI configuration and database files
//  3. Copying Nginx configuration directory
//  4. Creating individual ZIP archives for each component
//  5. Calculating cryptographic hashes for integrity verification
//  6. Encrypting all components with AES encryption
//  7. Creating final encrypted archive in memory
//
// Returns:
//   - BackupResult: Complete backup data including encrypted content and keys
//   - error: CosyError if any step of the backup process fails
func Backup() (Result, error) {
	// Generate timestamp for unique backup identification
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("backup-%s.zip", timestamp)

	// Generate cryptographic keys for AES encryption
	key, err := GenerateAESKey()
	if err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrGenerateAESKey, err.Error())
	}

	iv, err := GenerateIV()
	if err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrGenerateIV, err.Error())
	}

	// Create temporary directory for staging backup files
	tempDir, err := os.MkdirTemp("", "nginx-ui-backup-*")
	if err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateTempDir, err.Error())
	}
	defer os.RemoveAll(tempDir) // Ensure cleanup of temporary files

	// Create subdirectories for organizing backup components
	nginxUITempDir := filepath.Join(tempDir, NginxUIDir)
	nginxTempDir := filepath.Join(tempDir, NginxDir)
	if err := os.MkdirAll(nginxUITempDir, 0755); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateTempSubDir, err.Error())
	}
	if err := os.MkdirAll(nginxTempDir, 0755); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateTempSubDir, err.Error())
	}

	// Stage Nginx UI configuration and database files
	if err := backupNginxUIFiles(nginxUITempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrBackupNginxUI, err.Error())
	}

	// Stage Nginx configuration files
	if err := backupNginxFiles(nginxTempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrBackupNginx, err.Error())
	}

	// Create individual ZIP archives for each component
	nginxUIZipPath := filepath.Join(tempDir, NginxUIZipName)
	nginxZipPath := filepath.Join(tempDir, NginxZipName)

	// Compress Nginx UI files into archive
	if err := createZipArchive(nginxUIZipPath, nginxUITempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	// Compress Nginx configuration files into archive
	if err := createZipArchive(nginxZipPath, nginxTempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	// Calculate cryptographic hashes for integrity verification
	nginxUIHash, err := calculateFileHash(nginxUIZipPath)
	if err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCalculateHash, err.Error())
	}

	nginxHash, err := calculateFileHash(nginxZipPath)
	if err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCalculateHash, err.Error())
	}

	// Gather version information for backup metadata
	versionInfo := version.GetVersionInfo()

	// Create hash verification file with metadata
	hashInfo := HashInfo{
		NginxUIHash: nginxUIHash,
		NginxHash:   nginxHash,
		Timestamp:   timestamp,
		Version:     versionInfo.Version,
	}

	// Write hash information to verification file
	hashInfoPath := filepath.Join(tempDir, HashInfoFile)
	if err := writeHashInfoFile(hashInfoPath, hashInfo); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateHashFile, err.Error())
	}

	// Encrypt all backup components for security
	if err := encryptFile(hashInfoPath, key, iv); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrEncryptFile, HashInfoFile)
	}

	if err := encryptFile(nginxUIZipPath, key, iv); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrEncryptNginxUIDir, err.Error())
	}

	if err := encryptFile(nginxZipPath, key, iv); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrEncryptNginxDir, err.Error())
	}

	// Clean up unencrypted directories to prevent duplication in final archive
	if err := os.RemoveAll(nginxUITempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCleanupTempDir, err.Error())
	}
	if err := os.RemoveAll(nginxTempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCleanupTempDir, err.Error())
	}

	// Create final encrypted backup archive in memory
	var buffer bytes.Buffer
	if err := createZipArchiveToBuffer(&buffer, tempDir); err != nil {
		return Result{}, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	// Encode encryption keys as base64 for safe transmission/storage
	keyBase64 := base64.StdEncoding.EncodeToString(key)
	ivBase64 := base64.StdEncoding.EncodeToString(iv)

	// Assemble final backup result
	result := Result{
		BackupContent: buffer.Bytes(),
		BackupName:    backupName,
		AESKey:        keyBase64,
		AESIv:         ivBase64,
	}

	logger.Infof("Backup created successfully: %s (size: %d bytes)", backupName, len(buffer.Bytes()))
	return result, nil
}
