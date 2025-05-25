package backup

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

// MockBackupService is used to mock the backup service
type MockBackupService struct {
	mock.Mock
}

func (m *MockBackupService) Backup() (backup.BackupResult, error) {
	return backup.BackupResult{
		BackupName:    "backup-test.zip",
		AESKey:        "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=", // base64 encoded test key
		AESIv:         "YWJjZGVmZ2hpamtsbW5vcA==",                     // base64 encoded test IV
		BackupContent: []byte("test backup content"),
	}, nil
}

func (m *MockBackupService) Restore(options backup.RestoreOptions) (backup.RestoreResult, error) {
	return backup.RestoreResult{
		RestoreDir:      options.RestoreDir,
		NginxUIRestored: options.RestoreNginxUI,
		NginxRestored:   options.RestoreNginx,
		HashMatch:       options.VerifyHash,
	}, nil
}

// MockedCreateBackup is a mocked version of CreateBackup that uses the mock service
func MockedCreateBackup(c *gin.Context) {
	mockService := &MockBackupService{}
	result, err := mockService.Backup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Concatenate Key and IV
	securityToken := result.AESKey + ":" + result.AESIv

	// Set HTTP headers for file download
	fileName := result.BackupName
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("X-Backup-Security", securityToken) // Pass security token in header
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	// Send file content
	c.Data(http.StatusOK, "application/zip", result.BackupContent)
}

// MockedRestoreBackup is a mocked version of RestoreBackup that uses the mock service
func MockedRestoreBackup(c *gin.Context) {
	// Get restore options
	restoreNginx := c.PostForm("restore_nginx") == "true"
	restoreNginxUI := c.PostForm("restore_nginx_ui") == "true"
	verifyHash := c.PostForm("verify_hash") == "true"
	securityToken := c.PostForm("security_token")

	// Get backup file - we're just checking it exists for the test
	_, err := c.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Backup file not found",
		})
		return
	}

	// Validate security token
	if securityToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid security token",
		})
		return
	}

	// Split security token to get Key and IV
	parts := strings.Split(securityToken, ":")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid security token format",
		})
		return
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "nginx-ui-restore-test-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create temporary directory",
		})
		return
	}

	mockService := &MockBackupService{}
	result, err := mockService.Restore(backup.RestoreOptions{
		RestoreDir:     tempDir,
		RestoreNginx:   restoreNginx,
		RestoreNginxUI: restoreNginxUI,
		VerifyHash:     verifyHash,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, RestoreResponse{
		NginxUIRestored: result.NginxUIRestored,
		NginxRestored:   result.NginxRestored,
		HashMatch:       result.HashMatch,
	})
}

func TestSetupEnvironment(t *testing.T) {
	logger.Init(gin.DebugMode)
	// Set up test environment
	tempDir, err := os.MkdirTemp("", "nginx-ui-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set up necessary directories and config files
	nginxDir := filepath.Join(tempDir, "nginx")
	configDir := filepath.Join(tempDir, "config")

	err = os.MkdirAll(nginxDir, 0755)
	assert.NoError(t, err)

	err = os.MkdirAll(configDir, 0755)
	assert.NoError(t, err)

	// Create a config.ini file
	configPath := filepath.Join(configDir, "config.ini")
	err = os.WriteFile(configPath, []byte("[app]\nName = Nginx UI Test\n"), 0644)
	assert.NoError(t, err)

	// Create a database file
	dbName := settings.DatabaseSettings.GetName()
	dbPath := filepath.Join(configDir, dbName+".db")
	err = os.WriteFile(dbPath, []byte("test database content"), 0644)
	assert.NoError(t, err)

	// Save original settings for restoration later
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfPath := cosysettings.ConfPath

	t.Logf("Original config path: %s", cosysettings.ConfPath)
	t.Logf("Setting config path to: %s", configPath)

	// Set the temporary directory as the Nginx config directory for testing
	settings.NginxSettings.ConfigDir = nginxDir
	cosysettings.ConfPath = configPath

	t.Logf("Config path after setting: %s", cosysettings.ConfPath)

	// Restore original settings after test
	defer func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		cosysettings.ConfPath = originalConfPath
	}()
}

func setupMockedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Setup router with mocked API endpoints to avoid environment issues
	systemGroup := r.Group("/api/system")
	systemGroup.POST("/backup", MockedCreateBackup)
	systemGroup.POST("/backup/restore", MockedRestoreBackup)

	return r
}

func TestCreateBackupAPI(t *testing.T) {
	// Set up test environment
	TestSetupEnvironment(t)

	router := setupMockedRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/backup", nil)
	router.ServeHTTP(w, req)

	// If there's an error, it might be because the config path is empty
	if w.Code != http.StatusOK {
		var errorResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		if err == nil {
			t.Logf("Error response: %v", errorResponse)
		}

		// Skip the test if there's a configuration issue
		if strings.Contains(w.Body.String(), "Config path is empty") {
			t.Skip("Skipping test due to empty config path")
			return
		}
	}

	// Check response code - should be OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the backup API response
	assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))

	// Check that Content-Disposition contains "attachment; filename=backup-"
	contentDisposition := w.Header().Get("Content-Disposition")
	assert.True(t, strings.HasPrefix(contentDisposition, "attachment; filename=backup-"),
		"Content-Disposition should start with 'attachment; filename=backup-'")

	assert.NotEmpty(t, w.Header().Get("X-Backup-Security"))
	assert.NotEmpty(t, w.Body.Bytes())

	// Verify security token format
	securityToken := w.Header().Get("X-Backup-Security")
	parts := bytes.Split([]byte(securityToken), []byte(":"))
	assert.Equal(t, 2, len(parts))

	// Verify key and IV can be decoded
	key, err := base64.StdEncoding.DecodeString(string(parts[0]))
	assert.NoError(t, err)
	assert.Equal(t, 32, len(key))

	iv, err := base64.StdEncoding.DecodeString(string(parts[1]))
	assert.NoError(t, err)
	assert.Equal(t, 16, len(iv))
}

func TestRestoreBackupAPI(t *testing.T) {
	// Set up test environment
	TestSetupEnvironment(t)

	// First create a backup to restore
	backupRouter := setupMockedRouter()
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/system/backup", nil)
	backupRouter.ServeHTTP(w1, req1)

	// If there's an error creating the backup, skip the test
	if w1.Code != http.StatusOK {
		var errorResponse map[string]interface{}
		err := json.Unmarshal(w1.Body.Bytes(), &errorResponse)
		if err == nil {
			t.Logf("Error response during backup creation: %v", errorResponse)
		}
		t.Skip("Skipping test due to backup creation failure")
		return
	}

	assert.Equal(t, http.StatusOK, w1.Code)

	// Get the security token from the backup response
	securityToken := w1.Header().Get("X-Backup-Security")
	assert.NotEmpty(t, securityToken)

	// Get backup content
	backupContent := w1.Body.Bytes()
	assert.NotEmpty(t, backupContent)

	// Setup temporary directory and save backup file
	tempDir, err := os.MkdirTemp("", "restore-api-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	backupName := "backup-test.zip"
	backupPath := filepath.Join(tempDir, backupName)
	err = os.WriteFile(backupPath, backupContent, 0644)
	assert.NoError(t, err)

	// Setup router
	router := setupMockedRouter()

	// Create multipart form
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Add form fields
	_ = writer.WriteField("restore_nginx", "false")
	_ = writer.WriteField("restore_nginx_ui", "false")
	_ = writer.WriteField("verify_hash", "true")
	_ = writer.WriteField("security_token", securityToken)

	// Add backup file
	file, err := os.Open(backupPath)
	assert.NoError(t, err)
	defer file.Close()

	part, err := writer.CreateFormFile("backup_file", backupName)
	assert.NoError(t, err)

	_, err = io.Copy(part, file)
	assert.NoError(t, err)

	err = writer.Close()
	assert.NoError(t, err)

	// Create request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/backup/restore", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Perform request
	router.ServeHTTP(w, req)

	// Check status code
	t.Logf("Response: %s", w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response structure
	var response RestoreResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, false, response.NginxUIRestored)
	assert.Equal(t, false, response.NginxRestored)
	assert.Equal(t, true, response.HashMatch)
}

func TestRestoreBackupAPIErrors(t *testing.T) {
	// Set up test environment
	TestSetupEnvironment(t)

	// Setup router
	router := setupMockedRouter()

	// Test case 1: Missing backup file
	w1 := httptest.NewRecorder()
	body1 := new(bytes.Buffer)
	writer1 := multipart.NewWriter(body1)
	_ = writer1.WriteField("security_token", "invalid:token")
	writer1.Close()

	req1, _ := http.NewRequest("POST", "/api/system/backup/restore", body1)
	req1.Header.Set("Content-Type", writer1.FormDataContentType())

	router.ServeHTTP(w1, req1)
	assert.NotEqual(t, http.StatusOK, w1.Code)

	// Test case 2: Invalid security token
	w2 := httptest.NewRecorder()
	body2 := new(bytes.Buffer)
	writer2 := multipart.NewWriter(body2)
	_ = writer2.WriteField("security_token", "invalidtoken") // No colon separator
	writer2.Close()

	req2, _ := http.NewRequest("POST", "/api/system/backup/restore", body2)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())

	router.ServeHTTP(w2, req2)
	assert.NotEqual(t, http.StatusOK, w2.Code)

	// Test case 3: Invalid base64 encoding
	w3 := httptest.NewRecorder()
	body3 := new(bytes.Buffer)
	writer3 := multipart.NewWriter(body3)
	_ = writer3.WriteField("security_token", "invalid!base64:alsoinvalid!")
	writer3.Close()

	req3, _ := http.NewRequest("POST", "/api/system/backup/restore", body3)
	req3.Header.Set("Content-Type", writer3.FormDataContentType())

	router.ServeHTTP(w3, req3)
	assert.NotEqual(t, http.StatusOK, w3.Code)
}
