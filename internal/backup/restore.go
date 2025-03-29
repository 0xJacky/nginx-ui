package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

// RestoreResult contains the results of a restore operation
type RestoreResult struct {
	RestoreDir      string
	NginxUIRestored bool
	NginxRestored   bool
	HashMatch       bool
}

// RestoreOptions contains options for restore operation
type RestoreOptions struct {
	BackupPath     string
	AESKey         []byte
	AESIv          []byte
	RestoreDir     string
	RestoreNginx   bool
	VerifyHash     bool
	RestoreNginxUI bool
}

// Restore restores data from a backup archive
func Restore(options RestoreOptions) (RestoreResult, error) {
	// Create restore directory if it doesn't exist
	if err := os.MkdirAll(options.RestoreDir, 0755); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateRestoreDir, err.Error())
	}

	// Extract main archive to restore directory
	if err := extractZipArchive(options.BackupPath, options.RestoreDir); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrExtractArchive, err.Error())
	}

	// Decrypt the extracted files
	hashInfoPath := filepath.Join(options.RestoreDir, HashInfoFile)
	nginxUIZipPath := filepath.Join(options.RestoreDir, NginxUIZipName)
	nginxZipPath := filepath.Join(options.RestoreDir, NginxZipName)

	// Decrypt hash info file
	if err := decryptFile(hashInfoPath, options.AESKey, options.AESIv); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrDecryptFile, HashInfoFile)
	}

	// Decrypt nginx-ui.zip
	if err := decryptFile(nginxUIZipPath, options.AESKey, options.AESIv); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrDecryptNginxUIDir, err.Error())
	}

	// Decrypt nginx.zip
	if err := decryptFile(nginxZipPath, options.AESKey, options.AESIv); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrDecryptNginxDir, err.Error())
	}

	// Extract zip files to subdirectories
	nginxUIDir := filepath.Join(options.RestoreDir, NginxUIDir)
	nginxDir := filepath.Join(options.RestoreDir, NginxDir)

	if err := os.MkdirAll(nginxUIDir, 0755); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateDir, nginxUIDir)
	}

	if err := os.MkdirAll(nginxDir, 0755); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateDir, nginxDir)
	}

	// Extract nginx-ui.zip to nginx-ui directory
	if err := extractZipArchive(nginxUIZipPath, nginxUIDir); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrExtractArchive, "nginx-ui.zip")
	}

	// Extract nginx.zip to nginx directory
	if err := extractZipArchive(nginxZipPath, nginxDir); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrExtractArchive, "nginx.zip")
	}

	result := RestoreResult{
		RestoreDir:      options.RestoreDir,
		NginxUIRestored: false,
		NginxRestored:   false,
		HashMatch:       false,
	}

	// Verify hashes if requested
	if options.VerifyHash {
		hashMatch, err := verifyHashes(options.RestoreDir, nginxUIZipPath, nginxZipPath)
		if err != nil {
			return result, cosy.WrapErrorWithParams(ErrVerifyHashes, err.Error())
		}
		result.HashMatch = hashMatch
	}

	// Restore nginx configs if requested
	if options.RestoreNginx {
		if err := restoreNginxConfigs(nginxDir); err != nil {
			return result, cosy.WrapErrorWithParams(ErrRestoreNginxConfigs, err.Error())
		}
		result.NginxRestored = true
	}

	// Restore nginx-ui config if requested
	if options.RestoreNginxUI {
		if err := restoreNginxUIConfig(nginxUIDir); err != nil {
			return result, cosy.WrapErrorWithParams(ErrBackupNginxUI, err.Error())
		}
		result.NginxUIRestored = true
	}

	return result, nil
}

// extractZipArchive extracts a zip archive to the specified directory
func extractZipArchive(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrOpenZipFile, err.Error())
	}
	defer reader.Close()

	for _, file := range reader.File {
		err := extractZipFile(file, destDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive
func extractZipFile(file *zip.File, destDir string) error {
	// Check for directory traversal elements in the file name
	if strings.Contains(file.Name, "..") {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, file.Name)
	}

	// Create directory path if needed
	filePath := filepath.Join(destDir, file.Name)

	// Ensure the resulting file path is within the destination directory
	destDirAbs, err := filepath.Abs(destDir)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, "cannot resolve destination path")
	}

	filePathAbs, err := filepath.Abs(filePath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, file.Name)
	}

	if !strings.HasPrefix(filePathAbs, destDirAbs+string(os.PathSeparator)) {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, file.Name)
	}

	if file.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, file.Mode()); err != nil {
			return cosy.WrapErrorWithParams(ErrCreateDir, filePath)
		}
		return nil
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return cosy.WrapErrorWithParams(ErrCreateParentDir, filePath)
	}

	// Check if this is a symlink by examining mode bits
	if file.Mode()&os.ModeSymlink != 0 {
		// Open source file in zip to read the link target
		srcFile, err := file.Open()
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenZipEntry, file.Name)
		}
		defer srcFile.Close()

		// Read the link target
		linkTargetBytes, err := io.ReadAll(srcFile)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrReadSymlink, file.Name)
		}
		linkTarget := string(linkTargetBytes)

		// Verify the link target doesn't escape the destination directory
		absLinkTarget := filepath.Clean(filepath.Join(filepath.Dir(filePath), linkTarget))
		if !strings.HasPrefix(absLinkTarget, destDirAbs+string(os.PathSeparator)) {
			return cosy.WrapErrorWithParams(ErrInvalidFilePath, linkTarget)
		}

		// Remove any existing file/link at the target path
		_ = os.Remove(filePath)

		// Create the symlink
		if err := os.Symlink(linkTarget, filePath); err != nil {
			return cosy.WrapErrorWithParams(ErrCreateSymlink, file.Name)
		}

		// Verify the resolved symlink path is within destination directory
		resolvedPath, err := filepath.EvalSymlinks(filePath)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrEvalSymlinks, filePath)
		}

		resolvedPathAbs, err := filepath.Abs(resolvedPath)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrInvalidFilePath, resolvedPath)
		}

		if !strings.HasPrefix(resolvedPathAbs, destDirAbs+string(os.PathSeparator)) {
			// Remove the symlink if it points outside the destination directory
			_ = os.Remove(filePath)
			return cosy.WrapErrorWithParams(ErrInvalidFilePath, resolvedPath)
		}

		return nil
	}

	// Create file
	destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateFile, filePath)
	}
	defer destFile.Close()

	// Open source file in zip
	srcFile, err := file.Open()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrOpenZipEntry, file.Name)
	}
	defer srcFile.Close()

	// Copy content
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return cosy.WrapErrorWithParams(ErrCopyContent, file.Name)
	}

	return nil
}

// verifyHashes verifies the hashes of the extracted zip files
func verifyHashes(restoreDir, nginxUIZipPath, nginxZipPath string) (bool, error) {
	hashFile := filepath.Join(restoreDir, HashInfoFile)
	hashContent, err := os.ReadFile(hashFile)
	if err != nil {
		return false, cosy.WrapErrorWithParams(ErrReadHashFile, err.Error())
	}

	hashInfo := parseHashInfo(string(hashContent))

	// Calculate hash for nginx-ui.zip
	nginxUIHash, err := calculateFileHash(nginxUIZipPath)
	if err != nil {
		return false, cosy.WrapErrorWithParams(ErrCalculateUIHash, err.Error())
	}

	// Calculate hash for nginx.zip
	nginxHash, err := calculateFileHash(nginxZipPath)
	if err != nil {
		return false, cosy.WrapErrorWithParams(ErrCalculateNginxHash, err.Error())
	}

	// Verify hashes
	return (hashInfo.NginxUIHash == nginxUIHash && hashInfo.NginxHash == nginxHash), nil
}

// parseHashInfo parses hash info from content string
func parseHashInfo(content string) HashInfo {
	info := HashInfo{}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "nginx-ui_hash":
			info.NginxUIHash = value
		case "nginx_hash":
			info.NginxHash = value
		case "timestamp":
			info.Timestamp = value
		case "version":
			info.Version = value
		}
	}

	return info
}

// restoreNginxConfigs restores nginx configuration files
func restoreNginxConfigs(nginxBackupDir string) error {
	destDir := nginx.GetConfPath()
	if destDir == "" {
		return ErrNginxConfigDirEmpty
	}

	// Remove all contents in the destination directory first
	// Read directory entries
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCopyNginxConfigDir, "failed to read directory: "+err.Error())
	}

	// Remove each entry
	for _, entry := range entries {
		entryPath := filepath.Join(destDir, entry.Name())
		err := os.RemoveAll(entryPath)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCopyNginxConfigDir, "failed to remove: "+err.Error())
		}
	}

	// Copy files from backup to nginx config directory
	if err := copyDirectory(nginxBackupDir, destDir); err != nil {
		return err
	}

	return nil
}

// restoreNginxUIConfig restores nginx-ui configuration files
func restoreNginxUIConfig(nginxUIBackupDir string) error {
	// Get config directory
	configDir := filepath.Dir(cosysettings.ConfPath)
	if configDir == "" {
		return ErrConfigPathEmpty
	}

	// Restore app.ini to the configured location
	srcConfigPath := filepath.Join(nginxUIBackupDir, "app.ini")
	if err := copyFile(srcConfigPath, cosysettings.ConfPath); err != nil {
		return err
	}

	// Restore database file if exists
	dbName := settings.DatabaseSettings.GetName()
	srcDBPath := filepath.Join(nginxUIBackupDir, dbName+".db")
	destDBPath := filepath.Join(configDir, dbName+".db")

	// Only attempt to copy if database file exists in backup
	if _, err := os.Stat(srcDBPath); err == nil {
		if err := copyFile(srcDBPath, destDBPath); err != nil {
			return err
		}
	}

	return nil
}
