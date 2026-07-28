package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

// RestoreResult contains the results of a restore operation
type RestoreResult struct {
	RestoreDir      string
	NginxUIRestored bool
	NginxRestored   bool
	HashMatch       bool
	TrustLevel      ManifestTrust
	SkippedSettings []string
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

	nginxUIZipPath := filepath.Join(options.RestoreDir, NginxUIZipName)
	nginxZipPath := filepath.Join(options.RestoreDir, NginxZipName)

	trustLevel, err := verifyBackupManifest(options.RestoreDir, options.AESKey)
	if err != nil {
		return RestoreResult{}, err
	}

	result := RestoreResult{
		RestoreDir:      options.RestoreDir,
		NginxUIRestored: false,
		NginxRestored:   false,
		HashMatch:       true,
		TrustLevel:      trustLevel,
	}

	nginxUIDir := filepath.Join(options.RestoreDir, NginxUIDir)
	nginxDir := filepath.Join(options.RestoreDir, NginxDir)

	if options.RestoreNginxUI {
		if err := decryptFile(nginxUIZipPath, options.AESKey, options.AESIv); err != nil {
			return result, cosy.WrapErrorWithParams(ErrDecryptNginxUIDir, err.Error())
		}
		if err := os.MkdirAll(nginxUIDir, 0o755); err != nil {
			return result, cosy.WrapErrorWithParams(ErrCreateDir, err.Error())
		}
		if err := extractZipArchive(nginxUIZipPath, nginxUIDir); err != nil {
			return result, cosy.WrapErrorWithParams(ErrExtractArchive, err.Error())
		}
	}

	if options.RestoreNginx {
		if err := decryptFile(nginxZipPath, options.AESKey, options.AESIv); err != nil {
			return result, cosy.WrapErrorWithParams(ErrDecryptNginxDir, err.Error())
		}
		if err := os.MkdirAll(nginxDir, 0o755); err != nil {
			return result, cosy.WrapErrorWithParams(ErrCreateDir, err.Error())
		}
		if err := extractNginxZipArchive(nginxZipPath, nginxDir, nginx.GetConfPath(), nginx.GetModulesPath()); err != nil {
			return result, cosy.WrapErrorWithParams(ErrExtractArchive, err.Error())
		}
	}

	var nginxPlan *stagedNginxRestore
	var nginxUIPlan *stagedNginxUIRestore
	if options.RestoreNginx {
		nginxPlan, err = prepareNginxConfigs(nginxDir)
		if err != nil {
			return result, cosy.WrapErrorWithParams(ErrRestoreNginxConfigs, err.Error())
		}
		defer nginxPlan.Cleanup()
	}
	if options.RestoreNginxUI {
		nginxUIPlan, err = prepareNginxUIConfig(nginxUIDir, trustLevel)
		if err != nil {
			return result, cosy.WrapErrorWithParams(ErrBackupNginxUI, err.Error())
		}
		defer nginxUIPlan.Cleanup()
	}

	// Every selected tree has now been extracted, validated, and staged. Keep
	// rollback snapshots for each live target until all replacements succeed.
	var applied []appliedReplacement
	rollback := func(cause error) error {
		rollbackErrors := []error{cause}
		for index := len(applied) - 1; index >= 0; index-- {
			if rollbackErr := applied[index].Rollback(); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, rollbackErr)
			}
		}
		return errors.Join(rollbackErrors...)
	}
	if nginxPlan != nil {
		change, applyErr := nginxPlan.Apply()
		if applyErr != nil {
			return result, cosy.WrapErrorWithParams(ErrRestoreNginxConfigs, rollback(applyErr).Error())
		}
		applied = append(applied, change)
	}
	if nginxUIPlan != nil {
		change, applyErr := nginxUIPlan.Apply()
		if applyErr != nil {
			return result, cosy.WrapErrorWithParams(ErrBackupNginxUI, rollback(applyErr).Error())
		}
		applied = append(applied, change)
	}
	for _, change := range applied {
		if commitErr := change.Commit(); commitErr != nil {
			logger.Warn("Clean restore rollback snapshot: ", commitErr)
		}
	}

	if nginxPlan != nil {
		result.NginxRestored = true
	}
	if nginxUIPlan != nil {
		result.NginxUIRestored = true
		result.SkippedSettings = nginxUIPlan.skippedSettings
	}

	return result, nil
}

type stagedNginxRestore struct {
	candidate   string
	destination string
	stageRoot   string
}

func prepareNginxConfigs(nginxBackupDir string) (*stagedNginxRestore, error) {
	destination := nginx.GetConfPath()
	if destination == "" {
		return nil, ErrNginxConfigDirEmpty
	}
	stageRoot, err := os.MkdirTemp(filepath.Dir(destination), ".nginx-ui-nginx-stage-*")
	if err != nil {
		return nil, err
	}
	plan := &stagedNginxRestore{
		candidate:   filepath.Join(stageRoot, "candidate"),
		destination: destination,
		stageRoot:   stageRoot,
	}
	if err := copyDirectory(nginxBackupDir, plan.candidate); err != nil {
		plan.Cleanup()
		return nil, err
	}
	return plan, nil
}

func (plan *stagedNginxRestore) Apply() (appliedReplacement, error) {
	logger.Infof("Starting Nginx config restore to %s", plan.destination)
	return applyDirectoryWithRollback(plan.candidate, plan.destination)
}

func (plan *stagedNginxRestore) Cleanup() {
	if plan != nil {
		_ = os.RemoveAll(plan.stageRoot)
	}
}

// restoreNginxConfigs restores nginx configuration files
func restoreNginxConfigs(nginxBackupDir string) error {
	plan, err := prepareNginxConfigs(nginxBackupDir)
	if err != nil {
		return err
	}
	defer plan.Cleanup()
	applied, err := plan.Apply()
	if err != nil {
		return err
	}
	return applied.Commit()
}

// cleanDirectoryPreservingStructure removes all files and subdirectories in a directory
// but preserves the directory structure itself and handles mount points correctly.
func cleanDirectoryPreservingStructure(dir string) error {
	logger.Infof("Cleaning directory: %s", dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if err := removeOrClearPath(path, entry.IsDir()); err != nil {
			return err
		}
	}

	logger.Infof("Successfully cleaned directory: %s", dir)
	return nil
}

// removeOrClearPath removes a path or clears it if it's a mount point
func removeOrClearPath(path string, isDir bool) error {
	// Try to remove the path first
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}

	// Handle removal failures
	if !isDeviceBusyError(err) {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	// Device busy - check if it's a mount point or directory
	if !isDir {
		return fmt.Errorf("file is busy and cannot be removed: %s: %w", path, err)
	}

	logger.Warnf("Path is busy (mount point): %s, clearing contents only", path)
	return clearDirectoryContents(path)
}

// isMountPoint checks if a path is a mount point by comparing device IDs
// or checking /proc/mounts on Linux systems
func isMountPoint(path string) bool {
	if isDeviceDifferent(path) {
		return true
	}

	return isInMountTable(path)
}

// isDeviceDifferent and isInMountTable are implemented in platform-specific files:
// - restore_unix.go for Linux/Unix systems
// - restore_windows.go for Windows systems

// unescapeOctal converts octal escape sequences like \040 to their character equivalents
func unescapeOctal(s string) string {
	var result strings.Builder

	for i := 0; i < len(s); i++ {
		if char, skip := tryParseOctal(s, i); skip > 0 {
			result.WriteByte(char)
			i += skip - 1 // -1 because loop will increment
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// tryParseOctal attempts to parse octal sequence at position i
// returns (char, skip) where skip > 0 if successful
func tryParseOctal(s string, i int) (byte, int) {
	if s[i] != '\\' || i+3 >= len(s) {
		return 0, 0
	}

	var char byte
	if _, err := fmt.Sscanf(s[i:i+4], "\\%03o", &char); err == nil {
		return char, 4
	}

	return 0, 0
}

// isDeviceBusyError checks if an error is a "device or resource busy" error
func isDeviceBusyError(err error) bool {
	if err == nil {
		return false
	}

	if errno, ok := err.(syscall.Errno); ok && errno == syscall.EBUSY {
		return true
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "device or resource busy") ||
		strings.Contains(errMsg, "resource busy")
}

// clearDirectoryContents removes all files and subdirectories within a directory
// but preserves the directory itself. This is useful for cleaning mount points.
func clearDirectoryContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if err := removeOrClearPath(path, entry.IsDir()); err != nil {
			return fmt.Errorf("clear %s: %w", path, err)
		}
	}

	return nil
}

type stagedNginxUIRestore struct {
	replacements    []stagedFileReplacement
	removals        []string
	skippedSettings []string
}

func (plan *stagedNginxUIRestore) Apply() (appliedReplacement, error) {
	return applyFilesWithRollback(plan.replacements, plan.removals)
}

func (plan *stagedNginxUIRestore) Cleanup() {
	if plan == nil {
		return
	}
	for _, replacement := range plan.replacements {
		_ = os.Remove(replacement.staged)
	}
}

func prepareNginxUIConfig(nginxUIBackupDir string, trustLevel ManifestTrust) (*stagedNginxUIRestore, error) {
	// Get config directory
	configDir := filepath.Dir(cosysettings.ConfPath)
	if configDir == "" {
		return nil, ErrConfigPathEmpty
	}

	srcConfigPath := filepath.Join(nginxUIBackupDir, "app.ini")
	preserveProtected := trustLevel == ManifestTrustPortable
	configContent, skippedSettings, err := settings.BuildRestoreConfig(srcConfigPath, cosysettings.ConfPath, preserveProtected)
	if err != nil {
		return nil, err
	}
	stagedConfig, err := stageBytes(cosysettings.ConfPath, configContent, 0o600)
	if err != nil {
		return nil, err
	}
	plan := &stagedNginxUIRestore{
		replacements:    []stagedFileReplacement{{destination: cosysettings.ConfPath, staged: stagedConfig}},
		skippedSettings: skippedSettings,
	}
	fail := func(cause error) (*stagedNginxUIRestore, error) {
		plan.Cleanup()
		return nil, cause
	}

	dbName, err := restoredDatabaseName(configContent)
	if err != nil {
		return fail(err)
	}
	srcDBPath := filepath.Join(nginxUIBackupDir, dbName+".db")
	destDBPath := filepath.Join(configDir, dbName+".db")

	plan.removals = []string{destDBPath + "-wal", destDBPath + "-shm"}
	if _, err := os.Stat(srcDBPath); err == nil {
		stagedDatabase, err := stageFile(srcDBPath, destDBPath, 0o600)
		if err != nil {
			return fail(err)
		}
		plan.replacements = append(plan.replacements, stagedFileReplacement{destination: destDBPath, staged: stagedDatabase})
		if preserveProtected {
			if err := invalidatePortableCredentials(stagedDatabase); err != nil {
				return fail(err)
			}
		}
		if err := validateSQLiteDatabase(stagedDatabase); err != nil {
			return fail(err)
		}
	} else if !os.IsNotExist(err) {
		return fail(err)
	}
	return plan, nil
}

// restoreNginxUIConfig restores nginx-ui configuration files.
func restoreNginxUIConfig(nginxUIBackupDir string, trustLevel ManifestTrust) ([]string, error) {
	plan, err := prepareNginxUIConfig(nginxUIBackupDir, trustLevel)
	if err != nil {
		return nil, err
	}
	defer plan.Cleanup()
	applied, err := plan.Apply()
	if err != nil {
		return nil, err
	}
	if err := applied.Commit(); err != nil {
		return nil, err
	}
	return plan.skippedSettings, nil
}

func restoredDatabaseName(configContent []byte) (string, error) {
	config, err := ini.Load(configContent)
	if err != nil {
		return "", err
	}
	var databaseSettings settings.Database
	if err := config.Section("database").StrictMapTo(&databaseSettings); err != nil {
		return "", fmt.Errorf("parse restored database settings: %w", err)
	}
	return databaseSettings.GetName(), nil
}
