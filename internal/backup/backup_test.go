package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	cosylogger "github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

func init() {
	// Initialize logging system to avoid nil pointer exceptions during tests
	cosylogger.Init("debug")

	// Clean up backup files at the start of tests
	cleanupBackupFiles()
}

// cleanupBackupFiles removes all backup files in the current directory
func cleanupBackupFiles() {
	// Get current directory
	dir, err := os.Getwd()
	if err != nil {
		return
	}

	// Delete all backup files
	matches, err := filepath.Glob(filepath.Join(dir, "backup-*.zip"))
	if err == nil {
		for _, file := range matches {
			os.Remove(file)
		}
	}
}

// setupTestEnvironment creates a temporary environment for testing
func setupTestEnvironment(t *testing.T) (string, func()) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "backup-test-*")
	assert.NoError(t, err)

	// Set up necessary directories
	nginxDir := filepath.Join(tempDir, "nginx")
	nginxUIDir := filepath.Join(tempDir, "nginx-ui")
	configDir := filepath.Join(tempDir, "config")
	backupDir := filepath.Join(tempDir, "backup")

	// Create directories
	for _, dir := range []string{nginxDir, nginxUIDir, configDir, backupDir} {
		err = os.MkdirAll(dir, 0755)
		assert.NoError(t, err)
	}

	// Create some test files
	testFiles := map[string]string{
		filepath.Join(nginxDir, "nginx.conf"):    "user nginx;\nworker_processes auto;\n",
		filepath.Join(nginxUIDir, "config.json"): `{"version": "1.0", "settings": {"theme": "dark"}}`,
	}

	for file, content := range testFiles {
		err = os.WriteFile(file, []byte(content), 0644)
		assert.NoError(t, err)
	}

	// Save original configuration
	origNginxConfigDir := settings.NginxSettings.ConfigDir
	origNginxUIConfigPath := cosysettings.ConfPath

	// Set test configuration
	settings.NginxSettings.ConfigDir = nginxDir
	cosysettings.ConfPath = filepath.Join(configDir, "config.ini")

	// Return cleanup function
	cleanup := func() {
		// Restore original configuration
		settings.NginxSettings.ConfigDir = origNginxConfigDir
		cosysettings.ConfPath = origNginxUIConfigPath

		// Delete temporary directory
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// Test backup and restore functionality
func TestBackupAndRestore(t *testing.T) {
	// Make sure backup files are cleaned up at the start and end of the test
	cleanupBackupFiles()
	defer cleanupBackupFiles()

	// Create test configuration
	tempDir, err := os.MkdirTemp("", "nginx-ui-backup-test-*")
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

	// Create test nginx config
	testNginxContent := []byte("server {\n    listen 80;\n    server_name example.com;\n}\n")
	err = os.WriteFile(filepath.Join(nginxConfigDir, "nginx.conf"), testNginxContent, 0644)
	assert.NoError(t, err)

	// Setup settings for testing
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

	// Save backup content to a temporary file for restore testing
	backupPath := filepath.Join(tempDir, result.BackupName)
	err = os.WriteFile(backupPath, result.BackupContent, 0644)
	assert.NoError(t, err)

	// Test restore functionality
	restoreDir, err := os.MkdirTemp("", "nginx-ui-restore-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(restoreDir)

	// Decode AES key and IV
	aesKey, err := DecodeFromBase64(result.AESKey)
	assert.NoError(t, err)
	aesIv, err := DecodeFromBase64(result.AESIv)
	assert.NoError(t, err)

	// Perform restore
	restoreResult, err := Restore(RestoreOptions{
		BackupPath:     backupPath,
		AESKey:         aesKey,
		AESIv:          aesIv,
		RestoreDir:     restoreDir,
		RestoreNginx:   true,
		RestoreNginxUI: true,
		VerifyHash:     true,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, restoreResult.RestoreDir)

	// Verify restored directories
	nginxUIDir := filepath.Join(restoreDir, NginxUIDir)
	nginxDir := filepath.Join(restoreDir, NginxDir)

	_, err = os.Stat(nginxUIDir)
	assert.NoError(t, err)
	_, err = os.Stat(nginxDir)
	assert.NoError(t, err)

	// Verify hash info exists
	_, err = os.Stat(filepath.Join(restoreDir, HashInfoFile))
	assert.NoError(t, err)
}

// Test AES encryption/decryption
func TestEncryptionDecryption(t *testing.T) {
	// Test data
	testData := []byte("This is a test message to encrypt and decrypt")

	// Create temp dir for testing
	testDir, err := os.MkdirTemp("", "nginx-ui-crypto-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Create test file
	testFile := filepath.Join(testDir, "test.txt")
	err = os.WriteFile(testFile, testData, 0644)
	assert.NoError(t, err)

	// Generate AES key and IV
	key, err := GenerateAESKey()
	assert.NoError(t, err)
	iv, err := GenerateIV()
	assert.NoError(t, err)

	// Test encrypt file
	err = encryptFile(testFile, key, iv)
	assert.NoError(t, err)

	// Read encrypted data
	encryptedData, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.NotEqual(t, string(testData), string(encryptedData))

	// Test decrypt file
	err = decryptFile(testFile, key, iv)
	assert.NoError(t, err)

	// Read decrypted data
	decryptedData, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, string(testData), string(decryptedData))
}

// Test AES direct encryption/decryption
func TestAESEncryptDecrypt(t *testing.T) {
	// Generate key and IV
	key, err := GenerateAESKey()
	assert.NoError(t, err)

	iv, err := GenerateIV()
	assert.NoError(t, err)

	// Test data
	original := []byte("This is a test message for encryption and decryption")

	// Encrypt
	encrypted, err := AESEncrypt(original, key, iv)
	assert.NoError(t, err)
	assert.NotEqual(t, original, encrypted)

	// Decrypt
	decrypted, err := AESDecrypt(encrypted, key, iv)
	assert.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

// Test Base64 encoding/decoding
func TestEncodeDecodeBase64(t *testing.T) {
	original := []byte("Test data for base64 encoding")

	// Encode
	encoded := EncodeToBase64(original)

	// Decode
	decoded, err := DecodeFromBase64(encoded)
	assert.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestGenerateAESKey(t *testing.T) {
	key, err := GenerateAESKey()
	assert.NoError(t, err)
	assert.Equal(t, 32, len(key))
}

func TestGenerateIV(t *testing.T) {
	iv, err := GenerateIV()
	assert.NoError(t, err)
	assert.Equal(t, 16, len(iv))
}

func TestEncryptDecryptFile(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "encrypt-file-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := []byte("This is test content for file encryption")
	err = os.WriteFile(testFile, testContent, 0644)
	assert.NoError(t, err)

	// Generate key and IV
	key, err := GenerateAESKey()
	assert.NoError(t, err)

	iv, err := GenerateIV()
	assert.NoError(t, err)

	// Encrypt file
	err = encryptFile(testFile, key, iv)
	assert.NoError(t, err)

	// Read encrypted content
	encryptedContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.NotEqual(t, testContent, encryptedContent)

	// Decrypt file
	err = decryptFile(testFile, key, iv)
	assert.NoError(t, err)

	// Read decrypted content
	decryptedContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, testContent, decryptedContent)
}

func TestBackupRestore(t *testing.T) {
	// Set up test environment
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a config.ini file since it's required for the test
	configDir := filepath.Join(tempDir, "config")
	configPath := filepath.Join(configDir, "config.ini")
	err := os.WriteFile(configPath, []byte("[app]\nName = Nginx UI Test\n"), 0644)
	assert.NoError(t, err)

	// Update Cosy settings path
	originalConfPath := cosysettings.ConfPath
	cosysettings.ConfPath = configPath
	defer func() {
		cosysettings.ConfPath = originalConfPath
	}()

	// Create backup
	backupResult, err := Backup()
	// If there's an error, log it but continue testing
	if err != nil {
		t.Logf("Backup failed with error: %v", err)
		t.Fail()
		return
	}

	assert.NotNil(t, backupResult.BackupContent)
	assert.NotEmpty(t, backupResult.BackupName)
	assert.NotEmpty(t, backupResult.AESKey)
	assert.NotEmpty(t, backupResult.AESIv)

	// Create temporary file for restore testing
	backupPath := filepath.Join(tempDir, backupResult.BackupName)
	err = os.WriteFile(backupPath, backupResult.BackupContent, 0644)
	assert.NoError(t, err)

	// Decode key and IV
	key, err := DecodeFromBase64(backupResult.AESKey)
	assert.NoError(t, err)

	iv, err := DecodeFromBase64(backupResult.AESIv)
	assert.NoError(t, err)

	// Create restore directory
	restoreDir := filepath.Join(tempDir, "restore")
	err = os.MkdirAll(restoreDir, 0755)
	assert.NoError(t, err)

	// Create restore options
	options := RestoreOptions{
		BackupPath: backupPath,
		AESKey:     key,
		AESIv:      iv,
		RestoreDir: restoreDir,
		VerifyHash: true,
		// Avoid modifying the system
		RestoreNginx:   false,
		RestoreNginxUI: false,
	}

	// Test restore
	result, err := Restore(options)
	if err != nil {
		t.Logf("Restore failed with error: %v", err)
		t.Fail()
		return
	}

	assert.Equal(t, restoreDir, result.RestoreDir)
	// If hash verification is enabled, check the result
	if options.VerifyHash {
		assert.True(t, result.HashMatch, "Hash verification should pass")
	}
}

func TestCreateZipArchive(t *testing.T) {
	// Create temp directories
	tempSourceDir, err := os.MkdirTemp("", "zip-source-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempSourceDir)

	// Create some test files
	testFiles := []string{"file1.txt", "file2.txt", "subdir/file3.txt"}
	testContent := []byte("Test content")

	for _, file := range testFiles {
		filePath := filepath.Join(tempSourceDir, file)
		dirPath := filepath.Dir(filePath)

		err = os.MkdirAll(dirPath, 0755)
		assert.NoError(t, err)

		err = os.WriteFile(filePath, testContent, 0644)
		assert.NoError(t, err)
	}

	// Create zip file
	zipPath := filepath.Join(tempSourceDir, "test.zip")
	err = createZipArchive(zipPath, tempSourceDir)
	assert.NoError(t, err)

	// Verify zip file was created
	_, err = os.Stat(zipPath)
	assert.NoError(t, err)

	// Extract to new directory to verify contents
	extractDir := filepath.Join(tempSourceDir, "extract")
	err = os.MkdirAll(extractDir, 0755)
	assert.NoError(t, err)

	err = extractZipArchive(zipPath, extractDir)
	assert.NoError(t, err)

	// Verify extracted files
	for _, file := range testFiles {
		extractedPath := filepath.Join(extractDir, file)
		content, err := os.ReadFile(extractedPath)
		assert.NoError(t, err)
		assert.Equal(t, testContent, content)
	}
}

func TestHashCalculation(t *testing.T) {
	// Create temp file
	tempFile, err := os.CreateTemp("", "hash-test-*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write content
	testContent := []byte("Test content for hash calculation")
	_, err = tempFile.Write(testContent)
	assert.NoError(t, err)
	tempFile.Close()

	// Calculate hash
	hash, err := calculateFileHash(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Calculate again to verify consistency
	hash2, err := calculateFileHash(tempFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, hash, hash2)

	// Modify file and check hash changes
	err = os.WriteFile(tempFile.Name(), []byte("Modified content"), 0644)
	assert.NoError(t, err)

	hash3, err := calculateFileHash(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEqual(t, hash, hash3)
}
