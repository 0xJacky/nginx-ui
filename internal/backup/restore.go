package backup

import (
	"archive/zip"
	"fmt"
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
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrDecryptFile, err.Error())
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
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateDir, err.Error())
	}

	if err := os.MkdirAll(nginxDir, 0755); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateDir, err.Error())
	}

	// Extract nginx-ui.zip to nginx-ui directory
	if err := extractZipArchive(nginxUIZipPath, nginxUIDir); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrExtractArchive, err.Error())
	}

	// Extract nginx.zip to nginx directory
	if err := extractZipArchive(nginxZipPath, nginxDir); err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrExtractArchive, err.Error())
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
		return cosy.WrapErrorWithParams(ErrOpenZipFile, fmt.Sprintf("failed to open zip file %s: %v", zipPath, err))
	}
	defer reader.Close()

	for _, file := range reader.File {
		err := extractZipFile(file, destDir)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrExtractArchive, fmt.Sprintf("failed to extract file %s: %v", file.Name, err))
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive
func extractZipFile(file *zip.File, destDir string) error {
	// Check for directory traversal elements in the file name
	if strings.Contains(file.Name, "..") {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("file name contains directory traversal: %s", file.Name))
	}

	// Clean and normalize the file path
	cleanName := filepath.Clean(file.Name)
	if cleanName == "." || cleanName == ".." {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("invalid file name after cleaning: %s", file.Name))
	}

	// Create directory path if needed
	filePath := filepath.Join(destDir, cleanName)

	// Ensure the resulting file path is within the destination directory
	destDirAbs, err := filepath.Abs(destDir)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("cannot resolve destination path %s: %v", destDir, err))
	}

	filePathAbs, err := filepath.Abs(filePath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("cannot resolve file path %s: %v", filePath, err))
	}

	// Check if the file path is within the destination directory
	if !strings.HasPrefix(filePathAbs, destDirAbs+string(os.PathSeparator)) {
		return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("file path %s is outside destination directory %s", filePathAbs, destDirAbs))
	}

	if file.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, file.Mode()); err != nil {
			return cosy.WrapErrorWithParams(ErrCreateDir, fmt.Sprintf("failed to create directory %s: %v", filePath, err))
		}
		return nil
	}

	// Create parent directory if needed
	parentDir := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return cosy.WrapErrorWithParams(ErrCreateParentDir, fmt.Sprintf("failed to create parent directory %s: %v", parentDir, err))
	}

	// Check if this is a symlink by examining mode bits
	if file.Mode()&os.ModeSymlink != 0 {
		// Open source file in zip to read the link target
		srcFile, err := file.Open()
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenZipEntry, fmt.Sprintf("failed to open symlink source %s: %v", file.Name, err))
		}
		defer srcFile.Close()

		// Read the link target
		linkTargetBytes, err := io.ReadAll(srcFile)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrReadSymlink, fmt.Sprintf("failed to read symlink target for %s: %v", file.Name, err))
		}
		linkTarget := string(linkTargetBytes)

		// Clean and normalize the link target
		cleanLinkTarget := filepath.Clean(linkTarget)
		if cleanLinkTarget == "." || cleanLinkTarget == ".." {
			return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("invalid symlink target: %s", linkTarget))
		}

		// Get nginx modules path
		modulesPath := nginx.GetModulesPath()

		// Handle system directory symlinks
		if strings.HasPrefix(cleanLinkTarget, modulesPath) {
			// For nginx modules, we'll create a relative symlink to the modules directory
			relPath, err := filepath.Rel(filepath.Dir(filePath), modulesPath)
			if err != nil {
				return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("failed to convert modules path to relative: %v", err))
			}
			cleanLinkTarget = relPath
		} else if filepath.IsAbs(cleanLinkTarget) {
			// For other absolute paths, we'll create a directory instead of a symlink
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return cosy.WrapErrorWithParams(ErrCreateDir, fmt.Sprintf("failed to create directory %s: %v", filePath, err))
			}
			return nil
		}

		// Verify the link target doesn't escape the destination directory
		absLinkTarget := filepath.Clean(filepath.Join(filepath.Dir(filePath), cleanLinkTarget))
		if !strings.HasPrefix(absLinkTarget, destDirAbs+string(os.PathSeparator)) {
			// For nginx modules, we'll create a directory instead of a symlink
			if strings.HasPrefix(linkTarget, modulesPath) {
				if err := os.MkdirAll(filePath, 0755); err != nil {
					return cosy.WrapErrorWithParams(ErrCreateDir, fmt.Sprintf("failed to create modules directory %s: %v", filePath, err))
				}
				return nil
			}
			return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("symlink target %s is outside destination directory %s", absLinkTarget, destDirAbs))
		}

		// Remove any existing file/link at the target path
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			// Ignoring error, continue creating symlink
		}

		// Create the symlink
		if err := os.Symlink(cleanLinkTarget, filePath); err != nil {
			return cosy.WrapErrorWithParams(ErrCreateSymlink, fmt.Sprintf("failed to create symlink %s -> %s: %v", filePath, cleanLinkTarget, err))
		}

		// Verify the resolved symlink path is within destination directory
		resolvedPath, err := filepath.EvalSymlinks(filePath)
		if err != nil {
			// If we can't resolve the symlink, it's not a critical error
			// Just continue
			return nil
		}

		resolvedPathAbs, err := filepath.Abs(resolvedPath)
		if err != nil {
			// Not a critical error, continue
			return nil
		}

		if !strings.HasPrefix(resolvedPathAbs, destDirAbs+string(os.PathSeparator)) {
			// Remove the symlink if it points outside the destination directory
			_ = os.Remove(filePath)
			return cosy.WrapErrorWithParams(ErrInvalidFilePath, fmt.Sprintf("resolved symlink path %s is outside destination directory %s", resolvedPathAbs, destDirAbs))
		}

		return nil
	}

	// Create file
	destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateFile, fmt.Sprintf("failed to create file %s: %v", filePath, err))
	}
	defer destFile.Close()

	// Open source file in zip
	srcFile, err := file.Open()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrOpenZipEntry, fmt.Sprintf("failed to open zip entry %s: %v", file.Name, err))
	}
	defer srcFile.Close()

	// Copy content
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return cosy.WrapErrorWithParams(ErrCopyContent, fmt.Sprintf("failed to copy content for file %s: %v", file.Name, err))
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
