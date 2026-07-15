package model

import (
	"strings"
	"time"
	"unicode"
)

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeNginxAndNginxUI BackupType = "nginx_and_nginx_ui"
	BackupTypeCustomDir       BackupType = "custom_dir"
)

// StorageType represents where the backup is stored
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
)

// BackupStatus represents the status of the last backup
type BackupStatus string

const (
	BackupStatusPending BackupStatus = "pending"
	BackupStatusSuccess BackupStatus = "success"
	BackupStatusFailed  BackupStatus = "failed"
)

// AutoBackup represents an automatic backup configuration
type AutoBackup struct {
	Model
	Name             string       `json:"name" gorm:"not null;comment:Backup task name"`
	BackupType       BackupType   `json:"backup_type" gorm:"index;not null;comment:Type of backup"`
	StorageType      StorageType  `json:"storage_type" gorm:"index;not null;comment:Storage type (local/s3)"`
	BackupPath       string       `json:"backup_path" gorm:"comment:Custom directory path for backup"`
	StoragePath      string       `json:"storage_path" gorm:"not null;comment:Storage destination path"`
	CronExpression   string       `json:"cron_expression" gorm:"not null;comment:Cron expression for scheduling"`
	Enabled          bool         `json:"enabled" gorm:"index;default:true;comment:Whether the backup task is enabled"`
	LastBackupTime   *time.Time   `json:"last_backup_time" gorm:"comment:Last backup execution time"`
	LastBackupStatus BackupStatus `json:"last_backup_status" gorm:"default:'pending';comment:Status of last backup"`
	LastBackupError  string       `json:"last_backup_error" gorm:"comment:Error message from last backup if failed"`

	// S3 Configuration (only used when StorageType is S3)
	S3Endpoint        string `json:"s3_endpoint" gorm:"comment:S3 endpoint URL"`
	S3AccessKeyID     string `json:"s3_access_key_id" gorm:"comment:S3 access key ID;serializer:json[aes]"`
	S3SecretAccessKey string `json:"s3_secret_access_key" gorm:"comment:S3 secret access key;serializer:json[aes]"`
	S3Bucket          string `json:"s3_bucket" gorm:"comment:S3 bucket name"`
	S3Region          string `json:"s3_region" gorm:"comment:S3 region"`
}

func (a *AutoBackup) GetName() string {
	return SafeAutoBackupName(a.Name)
}

func SafeAutoBackupName(name string) string {
	const fallbackName = "backup"

	var builder strings.Builder
	lastUnderscore := false

	for _, r := range strings.TrimSpace(name) {
		if isSafeAutoBackupNameRune(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}

		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}

	safeName := builder.String()
	for strings.Contains(safeName, "..") {
		safeName = strings.ReplaceAll(safeName, "..", ".")
	}

	safeName = strings.Trim(safeName, "._-")
	if safeName == "" {
		return fallbackName
	}

	if isReservedWindowsFilename(safeName) {
		return "_" + safeName
	}

	return safeName
}

func isSafeAutoBackupNameRune(r rune) bool {
	return r == '.' || r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isReservedWindowsFilename(name string) bool {
	baseName := name
	if dotIndex := strings.IndexByte(baseName, '.'); dotIndex >= 0 {
		baseName = baseName[:dotIndex]
	}

	switch strings.ToUpper(baseName) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}
