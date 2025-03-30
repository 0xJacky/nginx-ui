package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

// TestBackupVersion verifies that the backup file contains correct version information
func TestBackupVersion(t *testing.T) {
	// Make sure backup files are cleaned up at the start and end of the test
	cleanupBackupFiles()
	defer cleanupBackupFiles()

	// Create test configuration
	tempDir, err := os.MkdirTemp("", "nginx-ui-backup-version-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create config file
	configPath := filepath.Join(tempDir, "config.ini")
	testConfig := []byte("[app]\nName = Nginx UI Test\n")
	err = os.WriteFile(configPath, testConfig, 0644)
	assert.NoError(t, err)

	// Create database file
	dbName := settings.DatabaseSettings.GetName()
	dbFile := dbName + ".db"
	dbPath := filepath.Join(tempDir, dbFile)
	testDB := []byte("CREATE TABLE users (id INT, name TEXT);")
	err = os.WriteFile(dbPath, testDB, 0644)
	assert.NoError(t, err)

	// Create nginx directory
	nginxConfigDir := filepath.Join(tempDir, "nginx")
	err = os.MkdirAll(nginxConfigDir, 0755)
	assert.NoError(t, err)

	// Create nginx config
	testNginxContent := []byte("server {\n    listen 80;\n    server_name example.com;\n}\n")
	err = os.WriteFile(filepath.Join(nginxConfigDir, "nginx.conf"), testNginxContent, 0644)
	assert.NoError(t, err)

	// Setup test environment
	originalConfPath := cosysettings.ConfPath
	originalNginxConfigDir := settings.NginxSettings.ConfigDir

	cosysettings.ConfPath = configPath
	settings.NginxSettings.ConfigDir = nginxConfigDir

	// Restore original settings after test
	defer func() {
		cosysettings.ConfPath = originalConfPath
		settings.NginxSettings.ConfigDir = originalNginxConfigDir
	}()

	// Run backup
	result, err := Backup()
	assert.NoError(t, err)
	assert.NotEmpty(t, result.BackupContent)
	assert.NotEmpty(t, result.BackupName)
	assert.NotEmpty(t, result.AESKey)
	assert.NotEmpty(t, result.AESIv)

	// Save backup content to temporary file for restore testing
	backupFile := filepath.Join(tempDir, result.BackupName)
	err = os.WriteFile(backupFile, result.BackupContent, 0644)
	assert.NoError(t, err)

	// Decode AES key and IV
	key, err := DecodeFromBase64(result.AESKey)
	assert.NoError(t, err)
	iv, err := DecodeFromBase64(result.AESIv)
	assert.NoError(t, err)

	// Use the Restore function to extract and verify
	restoreDir, err := os.MkdirTemp("", "nginx-ui-restore-version-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(restoreDir)

	restoreResult, err := Restore(RestoreOptions{
		BackupPath:     backupFile,
		AESKey:         key,
		AESIv:          iv,
		RestoreDir:     restoreDir,
		VerifyHash:     true,
		RestoreNginx:   false,
		RestoreNginxUI: false,
	})
	assert.NoError(t, err)
	assert.True(t, restoreResult.HashMatch, "Hash should match")

	// Check hash_info.txt file
	hashInfoPath := filepath.Join(restoreDir, HashInfoFile)
	hashInfoContent, err := os.ReadFile(hashInfoPath)
	assert.NoError(t, err)

	// Verify version information
	versionInfo := version.GetVersionInfo()
	expectedVersion := versionInfo.Version

	// Check if hash_info.txt contains version info
	hashInfoStr := string(hashInfoContent)
	t.Logf("Hash info content: %s", hashInfoStr)

	assert.True(t, strings.Contains(hashInfoStr, "version: "), "Hash info should contain version field")

	// Parse hash_info.txt content
	info := parseHashInfo(hashInfoStr)
	assert.Equal(t, expectedVersion, info.Version, "Backup version should match current version")
}
