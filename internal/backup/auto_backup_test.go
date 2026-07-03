package backup

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
)

func TestResolveAutoBackupOutputPathRejectsTraversalFilenames(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "parent traversal",
			filename: "../evil.zip",
		},
		{
			name:     "nested path",
			filename: "subdir/evil.zip",
		},
		{
			name:     "windows parent traversal",
			filename: `..\evil.zip`,
		},
		{
			name:     "unix absolute path",
			filename: "/tmp/evil.zip",
		},
		{
			name:     "windows absolute path",
			filename: `C:\evil.zip`,
		},
		{
			name:     "windows drive relative path",
			filename: "C:evil.zip",
		},
		{
			name:     "windows unc path",
			filename: `\\server\share\evil.zip`,
		},
		{
			name:     "cleaned path changes",
			filename: "./evil.zip",
		},
		{
			name:     "current directory",
			filename: ".",
		},
		{
			name:     "embedded traversal",
			filename: "evil..zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath, err := resolveAutoBackupOutputPath(baseDir, tt.filename)
			assert.Error(t, err)
			assert.Empty(t, outputPath)
		})
	}
}

func TestBuildAutoBackupOutputPathAllowsGeneratedAndSimpleFilenames(t *testing.T) {
	baseDir := t.TempDir()
	autoBackup := &model.AutoBackup{
		Name:        "daily backup",
		StorageType: model.StorageTypeLocal,
		StoragePath: baseDir,
	}

	generatedFilename := fmt.Sprintf("%s_%d.zip", autoBackup.GetName(), int64(123))
	generatedPath, err := buildAutoBackupOutputPath(autoBackup, generatedFilename)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(baseDir, "daily_backup_123.zip"), generatedPath)
	assertPathInsideBaseDir(t, baseDir, generatedPath)

	simplePath, err := resolveAutoBackupOutputPath(baseDir, "backup_123.zip")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(baseDir, "backup_123.zip"), simplePath)
	assertPathInsideBaseDir(t, baseDir, simplePath)
}

func TestValidateAutoBackupConfigRejectsUnsafeNames(t *testing.T) {
	tests := []struct {
		name       string
		backupName string
	}{
		{
			name:       "parent traversal",
			backupName: "../evil.zip",
		},
		{
			name:       "nested path",
			backupName: "subdir/evil.zip",
		},
		{
			name:       "windows parent traversal",
			backupName: `..\evil.zip`,
		},
		{
			name:       "unix absolute path",
			backupName: "/tmp/evil.zip",
		},
		{
			name:       "windows absolute path",
			backupName: `C:\evil.zip`,
		},
		{
			name:       "windows drive relative path",
			backupName: "C:evil.zip",
		},
		{
			name:       "embedded traversal",
			backupName: "evil..zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAutoBackupConfig(newS3AutoBackupConfig(tt.backupName))
			assert.Error(t, err)
		})
	}
}

func TestValidateAutoBackupConfigAllowsSimpleNames(t *testing.T) {
	tests := []string{
		"daily backup",
		"daily_backup",
		"backup.v1",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateAutoBackupConfig(newS3AutoBackupConfig(name))
			assert.NoError(t, err)
		})
	}
}

func newS3AutoBackupConfig(name string) *model.AutoBackup {
	return &model.AutoBackup{
		Name:              name,
		BackupType:        model.BackupTypeNginxAndNginxUI,
		StorageType:       model.StorageTypeS3,
		StoragePath:       "backups",
		S3AccessKeyID:     "test-access-key",
		S3SecretAccessKey: "test-secret-key",
		S3Bucket:          "test-bucket",
	}
}

func assertPathInsideBaseDir(t *testing.T, baseDir, targetPath string) {
	t.Helper()

	baseDirAbs, err := filepath.Abs(baseDir)
	assert.NoError(t, err)

	targetPathAbs, err := filepath.Abs(targetPath)
	assert.NoError(t, err)

	relPath, err := filepath.Rel(baseDirAbs, targetPathAbs)
	assert.NoError(t, err)
	assert.False(t, filepath.IsAbs(relPath))
	assert.False(t, relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)))
}
