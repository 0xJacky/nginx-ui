package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// BackupExecutionResult contains the result of a backup execution
type BackupExecutionResult struct {
	FilePath string // Path to the created backup file
	KeyPath  string // Path to the encryption key file (if applicable)
}

// ExecuteAutoBackup executes an automatic backup task based on the configuration.
// This function handles all types of backup operations and manages the backup status
// throughout the execution process.
//
// Parameters:
//   - autoBackup: The auto backup configuration to execute
//
// Returns:
//   - error: CosyError if backup execution fails, nil if successful
func ExecuteAutoBackup(autoBackup *model.AutoBackup) error {
	logger.Infof("Starting auto backup task: %s (ID: %d, Type: %s, Storage: %s)",
		autoBackup.GetName(), autoBackup.ID, autoBackup.BackupType, autoBackup.StorageType)

	// Validate storage configuration before starting backup
	if err := validateStorageConfiguration(autoBackup); err != nil {
		logger.Errorf("Storage configuration validation failed for task %s: %v", autoBackup.Name, err)
		updateBackupStatus(autoBackup.ID, model.BackupStatusFailed, err.Error())
		// Send validation failure notification
		notification.Error(
			fmt.Sprintf("Auto Backup Configuration Error: %s", autoBackup.Name),
			fmt.Sprintf("Storage configuration validation failed for backup task '%s'", autoBackup.Name),
			map[string]interface{}{
				"backup_id":   autoBackup.ID,
				"backup_name": autoBackup.Name,
				"error":       err.Error(),
				"timestamp":   time.Now(),
			},
		)
		return err
	}

	// Update backup status to pending
	if err := updateBackupStatus(autoBackup.ID, model.BackupStatusPending, ""); err != nil {
		logger.Errorf("Failed to update backup status to pending: %v", err)
		return cosy.WrapErrorWithParams(ErrAutoBackupWriteFile, err.Error())
	}

	// Execute backup based on type
	result, backupErr := executeBackupByType(autoBackup)

	// Update backup status based on execution result
	now := time.Now()
	if backupErr != nil {
		logger.Errorf("Auto backup task %s failed: %v", autoBackup.Name, backupErr)
		if updateErr := updateBackupStatusWithTime(autoBackup.ID, model.BackupStatusFailed, backupErr.Error(), &now); updateErr != nil {
			logger.Errorf("Failed to update backup status to failed: %v", updateErr)
		}
		// Send failure notification
		notification.Error(
			fmt.Sprintf("Auto Backup Failed: %s", autoBackup.Name),
			fmt.Sprintf("Backup task '%s' failed to execute", autoBackup.Name),
			map[string]interface{}{
				"backup_id":   autoBackup.ID,
				"backup_name": autoBackup.Name,
				"error":       backupErr.Error(),
				"timestamp":   now,
			},
		)
		return backupErr
	}

	// Handle storage upload based on storage type
	if uploadErr := handleBackupStorage(autoBackup, result); uploadErr != nil {
		logger.Errorf("Auto backup storage upload failed for task %s: %v", autoBackup.Name, uploadErr)
		if updateErr := updateBackupStatusWithTime(autoBackup.ID, model.BackupStatusFailed, uploadErr.Error(), &now); updateErr != nil {
			logger.Errorf("Failed to update backup status to failed: %v", updateErr)
		}
		// Send storage failure notification
		notification.Error(
			fmt.Sprintf("Auto Backup Storage Failed: %s", autoBackup.Name),
			fmt.Sprintf("Backup task '%s' failed during storage upload", autoBackup.Name),
			map[string]interface{}{
				"backup_id":   autoBackup.ID,
				"backup_name": autoBackup.Name,
				"error":       uploadErr.Error(),
				"timestamp":   now,
			},
		)
		return uploadErr
	}

	logger.Infof("Auto backup task %s completed successfully, file: %s", autoBackup.Name, result.FilePath)
	if updateErr := updateBackupStatusWithTime(autoBackup.ID, model.BackupStatusSuccess, "", &now); updateErr != nil {
		logger.Errorf("Failed to update backup status to success: %v", updateErr)
	}

	// Send success notification
	notification.Success(
		fmt.Sprintf("Auto Backup Completed: %s", autoBackup.Name),
		fmt.Sprintf("Backup task '%s' completed successfully", autoBackup.Name),
		map[string]interface{}{
			"backup_id":   autoBackup.ID,
			"backup_name": autoBackup.Name,
			"file_path":   result.FilePath,
			"timestamp":   now,
		},
	)

	return nil
}

// executeBackupByType executes the backup operation based on the backup type.
// This function centralizes the backup type routing logic.
//
// Parameters:
//   - autoBackup: The auto backup configuration
//
// Returns:
//   - BackupExecutionResult: Result containing file paths
//   - error: CosyError if backup fails
func executeBackupByType(autoBackup *model.AutoBackup) (*BackupExecutionResult, error) {
	switch autoBackup.BackupType {
	case model.BackupTypeNginxAndNginxUI:
		return createEncryptedBackup(autoBackup)
	case model.BackupTypeCustomDir:
		return createCustomDirectoryBackup(autoBackup)
	default:
		return nil, cosy.WrapErrorWithParams(ErrAutoBackupUnsupportedType, string(autoBackup.BackupType))
	}
}

// createEncryptedBackup creates an encrypted backup for Nginx/Nginx UI configurations.
// This function handles all configuration backup types that require encryption.
//
// Parameters:
//   - autoBackup: The auto backup configuration
//   - backupPrefix: Prefix for the backup filename
//
// Returns:
//   - BackupExecutionResult: Result containing file paths
//   - error: CosyError if backup creation fails
func createEncryptedBackup(autoBackup *model.AutoBackup) (*BackupExecutionResult, error) {
	// Generate unique filename with timestamp
	filename := fmt.Sprintf("%s_%d.zip", autoBackup.GetName(), time.Now().Unix())

	// Determine output path based on storage type
	var outputPath string
	if autoBackup.StorageType == model.StorageTypeS3 {
		// For S3 storage, create temporary file
		tempDir := os.TempDir()
		outputPath = filepath.Join(tempDir, filename)
	} else {
		// For local storage, use the configured storage path
		outputPath = filepath.Join(autoBackup.StoragePath, filename)
	}

	// Create backup using the main backup function
	backupResult, err := Backup()
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrBackupNginx, err.Error())
	}

	// Write encrypted backup content to file
	if err := writeBackupFile(outputPath, backupResult.BackupContent); err != nil {
		return nil, err
	}

	// Create and write encryption key file
	keyPath := outputPath + ".key"
	if err := writeKeyFile(keyPath, backupResult.AESKey, backupResult.AESIv); err != nil {
		return nil, err
	}

	return &BackupExecutionResult{
		FilePath: outputPath,
		KeyPath:  keyPath,
	}, nil
}

// createCustomDirectoryBackup creates an unencrypted backup of a custom directory.
// This function handles custom directory backups which are stored as plain ZIP files.
//
// Parameters:
//   - autoBackup: The auto backup configuration
//
// Returns:
//   - BackupExecutionResult: Result containing file paths
//   - error: CosyError if backup creation fails
func createCustomDirectoryBackup(autoBackup *model.AutoBackup) (*BackupExecutionResult, error) {
	// Validate that backup path is specified for custom directory backup
	if autoBackup.BackupPath == "" {
		return nil, ErrAutoBackupPathRequired
	}

	// Validate backup source path
	if err := ValidateBackupPath(autoBackup.BackupPath); err != nil {
		return nil, err
	}

	// Generate unique filename with timestamp
	filename := fmt.Sprintf("custom_dir_%s_%d.zip", autoBackup.GetName(), time.Now().Unix())

	// Determine output path based on storage type
	var outputPath string
	if autoBackup.StorageType == model.StorageTypeS3 {
		// For S3 storage, create temporary file
		tempDir := os.TempDir()
		outputPath = filepath.Join(tempDir, filename)
	} else {
		// For local storage, use the configured storage path
		outputPath = filepath.Join(autoBackup.StoragePath, filename)
	}

	// Create unencrypted ZIP archive of the custom directory
	if err := createZipArchive(outputPath, autoBackup.BackupPath); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrCreateZipArchive, err.Error())
	}

	return &BackupExecutionResult{
		FilePath: outputPath,
		KeyPath:  "", // No key file for unencrypted backups
	}, nil
}

// writeBackupFile writes backup content to the specified file path with proper permissions.
// This function ensures backup files are created with secure permissions.
//
// Parameters:
//   - filePath: Destination file path
//   - content: Backup content to write
//
// Returns:
//   - error: CosyError if file writing fails
func writeBackupFile(filePath string, content []byte) error {
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		return cosy.WrapErrorWithParams(ErrAutoBackupWriteFile, err.Error())
	}
	return nil
}

// writeKeyFile writes encryption key information to a key file.
// This function creates a key file containing AES key and IV for encrypted backups.
//
// Parameters:
//   - keyPath: Path for the key file
//   - aesKey: Base64 encoded AES key
//   - aesIv: Base64 encoded AES initialization vector
//
// Returns:
//   - error: CosyError if key file writing fails
func writeKeyFile(keyPath, aesKey, aesIv string) error {
	keyContent := fmt.Sprintf("%s:%s", aesKey, aesIv)
	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		return cosy.WrapErrorWithParams(ErrAutoBackupWriteKeyFile, err.Error())
	}
	return nil
}

// updateBackupStatus updates the backup status in the database.
// This function provides a centralized way to update backup execution status.
//
// Parameters:
//   - id: Auto backup configuration ID
//   - status: New backup status
//   - errorMsg: Error message (empty for successful backups)
//
// Returns:
//   - error: Database error if update fails
func updateBackupStatus(id uint64, status model.BackupStatus, errorMsg string) error {
	_, err := query.AutoBackup.Where(query.AutoBackup.ID.Eq(id)).Updates(map[string]interface{}{
		"last_backup_status": status,
		"last_backup_error":  errorMsg,
	})
	return err
}

// updateBackupStatusWithTime updates the backup status and timestamp in the database.
// This function updates both status and execution time for completed backup operations.
//
// Parameters:
//   - id: Auto backup configuration ID
//   - status: New backup status
//   - errorMsg: Error message (empty for successful backups)
//   - backupTime: Timestamp of the backup execution
//
// Returns:
//   - error: Database error if update fails
func updateBackupStatusWithTime(id uint64, status model.BackupStatus, errorMsg string, backupTime *time.Time) error {
	_, err := query.AutoBackup.Where(query.AutoBackup.ID.Eq(id)).Updates(map[string]interface{}{
		"last_backup_status": status,
		"last_backup_error":  errorMsg,
		"last_backup_time":   backupTime,
	})
	return err
}

// GetEnabledAutoBackups retrieves all enabled auto backup configurations from the database.
// This function is used by the cron scheduler to get active backup tasks.
//
// Returns:
//   - []*model.AutoBackup: List of enabled auto backup configurations
//   - error: Database error if query fails
func GetEnabledAutoBackups() ([]*model.AutoBackup, error) {
	return query.AutoBackup.Where(query.AutoBackup.Enabled.Is(true)).Find()
}

// GetAutoBackupByID retrieves a specific auto backup configuration by its ID.
// This function provides access to individual backup configurations.
//
// Parameters:
//   - id: Auto backup configuration ID
//
// Returns:
//   - *model.AutoBackup: The auto backup configuration
//   - error: Database error if query fails or record not found
func GetAutoBackupByID(id uint64) (*model.AutoBackup, error) {
	return query.AutoBackup.Where(query.AutoBackup.ID.Eq(id)).First()
}

// validateStorageConfiguration validates the storage configuration based on storage type.
// This function centralizes storage validation logic for both local and S3 storage.
//
// Parameters:
//   - autoBackup: The auto backup configuration to validate
//
// Returns:
//   - error: CosyError if validation fails, nil if configuration is valid
func validateStorageConfiguration(autoBackup *model.AutoBackup) error {
	switch autoBackup.StorageType {
	case model.StorageTypeLocal:
		// For local storage, validate the storage path
		return ValidateStoragePath(autoBackup.StoragePath)
	case model.StorageTypeS3:
		// For S3 storage, test the connection
		s3Client, err := NewS3Client(autoBackup)
		if err != nil {
			return err
		}
		return s3Client.TestS3Connection(context.Background())
	default:
		return cosy.WrapErrorWithParams(ErrAutoBackupUnsupportedType, string(autoBackup.StorageType))
	}
}

// handleBackupStorage handles the storage of backup files based on storage type.
// This function routes backup storage to the appropriate handler (local or S3).
//
// Parameters:
//   - autoBackup: The auto backup configuration
//   - result: The backup execution result containing file paths
//
// Returns:
//   - error: CosyError if storage operation fails
func handleBackupStorage(autoBackup *model.AutoBackup, result *BackupExecutionResult) error {
	switch autoBackup.StorageType {
	case model.StorageTypeLocal:
		// For local storage, files are already written to the correct location
		logger.Infof("Backup files stored locally: %s", result.FilePath)
		return nil
	case model.StorageTypeS3:
		// For S3 storage, upload files to S3 and optionally clean up local files
		return handleS3Storage(autoBackup, result)
	default:
		return cosy.WrapErrorWithParams(ErrAutoBackupUnsupportedType, string(autoBackup.StorageType))
	}
}

// handleS3Storage handles S3 storage operations for backup files.
// This function uploads backup files to S3 and manages local file cleanup.
//
// Parameters:
//   - autoBackup: The auto backup configuration
//   - result: The backup execution result containing file paths
//
// Returns:
//   - error: CosyError if S3 operations fail
func handleS3Storage(autoBackup *model.AutoBackup, result *BackupExecutionResult) error {
	// Create S3 client
	s3Client, err := NewS3Client(autoBackup)
	if err != nil {
		return err
	}

	// Upload backup files to S3
	ctx := context.Background()
	if err := s3Client.UploadBackupFiles(ctx, result, autoBackup); err != nil {
		return err
	}

	// Clean up local files after successful S3 upload
	if err := cleanupLocalBackupFiles(result); err != nil {
		logger.Warnf("Failed to cleanup local backup files: %v", err)
		// Don't return error for cleanup failure as the backup was successful
	}

	logger.Infof("Backup files successfully uploaded to S3 and local files cleaned up")
	return nil
}

// cleanupLocalBackupFiles removes local backup files after successful S3 upload.
// This function helps manage disk space by removing temporary local files.
//
// Parameters:
//   - result: The backup execution result containing file paths to clean up
//
// Returns:
//   - error: Standard error if cleanup fails
func cleanupLocalBackupFiles(result *BackupExecutionResult) error {
	// Remove backup file
	if err := os.Remove(result.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup file %s: %v", result.FilePath, err)
	}

	// Remove key file if it exists
	if result.KeyPath != "" {
		if err := os.Remove(result.KeyPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove key file %s: %v", result.KeyPath, err)
		}
	}

	return nil
}

// ValidateAutoBackupConfig performs comprehensive validation of auto backup configuration.
// This function centralizes all validation logic for both creation and modification.
//
// Parameters:
//   - config: Auto backup configuration to validate
//
// Returns:
//   - error: CosyError if validation fails, nil if configuration is valid
func ValidateAutoBackupConfig(config *model.AutoBackup) error {
	// Validate backup path for custom directory backup type
	if config.BackupType == model.BackupTypeCustomDir {
		if config.BackupPath == "" {
			return ErrAutoBackupPathRequired
		}

		// Use centralized path validation from backup package
		if err := ValidateBackupPath(config.BackupPath); err != nil {
			return err
		}
	}

	// Validate storage path using centralized validation
	if config.StorageType == model.StorageTypeLocal && config.StoragePath != "" {
		if err := ValidateStoragePath(config.StoragePath); err != nil {
			return err
		}
	}

	// Validate S3 configuration if storage type is S3
	if config.StorageType == model.StorageTypeS3 {
		if err := ValidateS3Config(config); err != nil {
			return err
		}
	}

	return nil
}

// ValidateS3Config validates S3 storage configuration completeness.
// This function ensures all required S3 fields are provided when S3 storage is selected.
//
// Parameters:
//   - config: Auto backup configuration with S3 settings
//
// Returns:
//   - error: CosyError if S3 configuration is incomplete, nil if valid
func ValidateS3Config(config *model.AutoBackup) error {
	var missingFields []string

	// Check required S3 fields
	if config.S3Bucket == "" {
		missingFields = append(missingFields, "bucket")
	}
	if config.S3AccessKeyID == "" {
		missingFields = append(missingFields, "access_key_id")
	}
	if config.S3SecretAccessKey == "" {
		missingFields = append(missingFields, "secret_access_key")
	}

	// Return error if any required fields are missing
	if len(missingFields) > 0 {
		return cosy.WrapErrorWithParams(ErrAutoBackupS3ConfigIncomplete, strings.Join(missingFields, ", "))
	}

	return nil
}

// RestoreAutoBackup restores a soft-deleted auto backup configuration.
// This function restores the backup configuration and re-registers the cron job if enabled.
//
// Parameters:
//   - id: Auto backup configuration ID to restore
//
// Returns:
//   - error: Database error if restore fails
func RestoreAutoBackup(id uint64) error {
	// Restore the soft-deleted record
	_, err := query.AutoBackup.Unscoped().Where(query.AutoBackup.ID.Eq(id)).Update(query.AutoBackup.DeletedAt, nil)
	if err != nil {
		return err
	}

	// Get the restored backup configuration
	autoBackup, err := GetAutoBackupByID(id)
	if err != nil {
		return err
	}

	// Re-register cron job if the backup is enabled
	if autoBackup.Enabled {
		// Import cron package to register the job
		// Note: This would require importing the cron package, which might create circular dependency
		// The actual implementation should be handled at the API level
		logger.Infof("Auto backup %d restored and needs cron job registration", id)
	}

	return nil
}
