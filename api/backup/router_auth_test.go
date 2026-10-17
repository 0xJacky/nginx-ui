package backup

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	internalbackup "github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internalSystem "github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type restoreAuthFixture struct {
	router   *gin.Engine
	admin    *model.User
	token    string
	database *gorm.DB
	backup   internalbackup.Result
}

func setupRestoreAuth(t *testing.T) restoreAuthFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	originalJWT := cosysettings.AppSettings.JwtSecret
	originalConfPath := cosysettings.ConfPath
	originalCrypto := settings.CryptoSettings.Secret
	originalNode := *settings.NodeSettings
	originalNginx := *settings.NginxSettings
	originalDatabase := *settings.DatabaseSettings
	t.Cleanup(func() {
		cache.Shutdown()
		model.Use(nil)
		cosysettings.AppSettings.JwtSecret = originalJWT
		cosysettings.ConfPath = originalConfPath
		settings.CryptoSettings.Secret = originalCrypto
		*settings.NodeSettings = originalNode
		*settings.NginxSettings = originalNginx
		*settings.DatabaseSettings = originalDatabase
	})
	cosysettings.AppSettings.JwtSecret = "restore-auth-jwt-secret"
	settings.CryptoSettings.Secret = "restore-auth-crypto-secret"
	settings.NodeSettings.InstanceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	settings.NodeSettings.Demo = false
	settings.DatabaseSettings.Name = "restore-auth"
	root := t.TempDir()
	cosysettings.ConfPath = filepath.Join(root, "app.ini")
	require.NoError(t, os.WriteFile(cosysettings.ConfPath, []byte("[app]\n[database]\nName = restore-auth\n"), 0o600))
	settings.NginxSettings.ConfigDir = filepath.Join(root, "nginx")
	settings.NginxSettings.ConfigPath = filepath.Join(settings.NginxSettings.ConfigDir, "nginx.conf")
	settings.NginxSettings.ContainerName = ""
	settings.NginxSettings.HostMode = ""
	require.NoError(t, os.MkdirAll(settings.NginxSettings.ConfigDir, 0o700))
	require.NoError(t, os.WriteFile(settings.NginxSettings.ConfigPath, []byte("events {}\nhttp {}\n"), 0o600))

	database, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.MCPServiceToken{}))
	model.Use(database)
	query.Use(database)
	query.SetDefault(database)
	admin := &model.User{Name: "restore-admin", Password: "existing-password-hash", Status: true, Language: "en"}
	require.NoError(t, database.Create(admin).Error)
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	archive, err := internalbackup.Backup()
	require.NoError(t, err)
	router := gin.New()
	InitRouter(router.Group("/api"))
	return restoreAuthFixture{router: router, admin: admin, token: payload.Token, database: database, backup: archive}
}

func restoreValidationRequest(t *testing.T, endpoint string, archive internalbackup.Result) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	upload, err := writer.CreateFormFile("backup_file", "backup.zip")
	require.NoError(t, err)
	_, err = upload.Write(archive.BackupContent)
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("security_token", archive.AESKey+":"+archive.AESIv))
	// Validate a real archive without replacing files or scheduling a restart.
	require.NoError(t, writer.WriteField("restore_nginx", "false"))
	require.NoError(t, writer.WriteField("restore_nginx_ui", "false"))
	require.NoError(t, writer.WriteField("verify_hash", "true"))
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, endpoint, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

type restoreBodyProbe struct {
	reads int
}

func (p *restoreBodyProbe) Read([]byte) (int, error) {
	p.reads++
	return 0, io.ErrUnexpectedEOF
}

func TestRestoreRejectsServiceTokenBeforeReadingArchive(t *testing.T) {
	fixture := setupRestoreAuth(t)
	_, token, err := internalmcp.CreateServiceToken("restore-writer", []string{model.APITokenScopeWrite}, nil, fixture.admin.ID)
	require.NoError(t, err)
	probe := &restoreBodyProbe{}
	request := httptest.NewRequest(http.MethodPost, "/api/restore", probe)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=restore-probe")
	recorder := httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.JSONEq(t, `{"message":"This operation requires an interactive administrator"}`, recorder.Body.String())
	require.Zero(t, probe.reads, "account-replacing archives must not be processed for service tokens")
	var existing model.User
	require.NoError(t, fixture.database.First(&existing, fixture.admin.ID).Error)
	require.Equal(t, fixture.admin.Name, existing.Name)
	require.Equal(t, fixture.admin.Password, existing.Password)
}

func TestRestoreKeepsInteractiveAndMFASessionFlows(t *testing.T) {
	fixture := setupRestoreAuth(t)
	request := restoreValidationRequest(t, "/api/restore", fixture.backup)
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	recorder := httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"hash_match":true`)

	require.NoError(t, fixture.database.Model(fixture.admin).Update("otp_secret", []byte("enabled-otp-secret")).Error)
	internaluser.InvalidateUserCache(fixture.admin.ID)
	probe := &restoreBodyProbe{}
	request = httptest.NewRequest(http.MethodPost, "/api/restore", probe)
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=restore-probe")
	recorder = httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, probe.reads)

	request = restoreValidationRequest(t, "/api/restore", fixture.backup)
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	request.Header.Set("X-Secure-Session-ID", internaluser.SetSecureSessionID(fixture.admin.ID))
	recorder = httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
}

func TestSetupRestoreKeepsInstallSecretAuthentication(t *testing.T) {
	fixture := setupRestoreAuth(t)
	cosysettings.AppSettings.JwtSecret = ""
	settings.NodeSettings.SkipInstallation = false
	internalSystem.SetInstallStartupTimeForTest(time.Now())
	t.Cleanup(func() {
		_ = internalSystem.CleanupInstallSecret()
		internalSystem.SetInstallStartupTimeForTest(time.Now())
	})
	require.NoError(t, internalSystem.EnsureInstallSecret())
	secret, err := os.ReadFile(internalSystem.InstallSecretPath())
	require.NoError(t, err)
	router := gin.New()
	InitSetupRouter(router.Group("/api/setup", middleware.SetupAuthRequired()))

	request := restoreValidationRequest(t, "/api/setup/restore", fixture.backup)
	request.Header.Set(internalSystem.InstallSecretHeaderName, strings.TrimSpace(string(secret)))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	probe := &restoreBodyProbe{}
	request = httptest.NewRequest(http.MethodPost, "/api/setup/restore", probe)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=restore-probe")
	request.Header.Set(internalSystem.InstallSecretHeaderName, "invalid-install-secret")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.NotEqual(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":40310`)
	require.Zero(t, probe.reads)
}

func TestPortableRestorePreservesInteractiveAccounts(t *testing.T) {
	setupRestoreAuth(t)
	databasePath := filepath.Join(filepath.Dir(cosysettings.ConfPath), settings.DatabaseSettings.GetName()+".db")
	source, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, source.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, password TEXT, otp_secret BLOB, recovery_codes BLOB)`).Error)
	require.NoError(t, source.Exec(`INSERT INTO users VALUES (1, 'migrated-admin', 'original-password-hash', x'01', x'02')`).Error)
	connection, err := source.DB()
	require.NoError(t, err)
	require.NoError(t, connection.Close())
	archive, err := internalbackup.Backup()
	require.NoError(t, err)
	archivePath := filepath.Join(t.TempDir(), "portable.zip")
	require.NoError(t, os.WriteFile(archivePath, archive.BackupContent, 0o600))
	key, err := base64.StdEncoding.DecodeString(archive.AESKey)
	require.NoError(t, err)
	iv, err := base64.StdEncoding.DecodeString(archive.AESIv)
	require.NoError(t, err)
	settings.CryptoSettings.Secret = "portable-restore-destination-secret"
	require.NoError(t, os.Remove(databasePath))

	// Exercise the real portable database replacement without the HTTP
	// handler's asynchronous process restart. All destinations are temporary.
	result, err := internalbackup.Restore(internalbackup.RestoreOptions{
		BackupPath: archivePath, AESKey: key, AESIv: iv,
		RestoreDir: t.TempDir(), RestoreNginxUI: true, VerifyHash: true,
	})
	require.NoError(t, err)
	require.True(t, result.NginxUIRestored)
	require.Equal(t, internalbackup.ManifestTrustPortable, result.TrustLevel)
	restored, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	require.NoError(t, err)
	restoredConnection, err := restored.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = restoredConnection.Close() })
	var account struct {
		Name          string
		Password      string
		OTPSecret     []byte
		RecoveryCodes []byte
	}
	require.NoError(t, restored.Table("users").First(&account).Error)
	require.Equal(t, "migrated-admin", account.Name)
	require.Equal(t, "original-password-hash", account.Password)
	require.Empty(t, account.OTPSecret)
	require.Empty(t, account.RecoveryCodes)
}
