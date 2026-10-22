package backup

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	userapi "github.com/0xJacky/Nginx-UI/api/user"
	internalbackup "github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type backupExportFixture struct {
	router                                 *gin.Engine
	admin                                  *model.User
	database                               *gorm.DB
	jwt, shortToken, writeToken, readToken string
}

func setupBackupExport(t *testing.T, withOTP bool) backupExportFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	previousJWT, previousConf := cosysettings.AppSettings.JwtSecret, cosysettings.ConfPath
	previousCrypto := settings.CryptoSettings.Secret
	previousNode, previousNginx, previousDatabase, previousAuth := *settings.NodeSettings, *settings.NginxSettings, *settings.DatabaseSettings, *settings.AuthSettings
	t.Cleanup(func() {
		cache.Shutdown()
		model.Use(nil)
		cosysettings.AppSettings.JwtSecret, cosysettings.ConfPath = previousJWT, previousConf
		settings.CryptoSettings.Secret = previousCrypto
		*settings.NodeSettings, *settings.NginxSettings, *settings.DatabaseSettings, *settings.AuthSettings = previousNode, previousNginx, previousDatabase, previousAuth
	})
	root := t.TempDir()
	cosysettings.AppSettings.JwtSecret = "backup-export-synthetic-jwt-secret"
	cosysettings.ConfPath = filepath.Join(root, "app.ini")
	settings.CryptoSettings.Secret = "backup-export-synthetic-crypto-secret"
	settings.DatabaseSettings.Name = "backup-export"
	settings.NodeSettings.InstanceID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	settings.NodeSettings.Secret = "backup-export-synthetic-node-secret"
	settings.NodeSettings.Demo = false
	settings.AuthSettings.MFARequired = false
	settings.NginxSettings.ConfigDir = filepath.Join(root, "nginx")
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx", "nginx.conf")
	settings.NginxSettings.HostMode = ""
	settings.NginxSettings.ContainerName = ""
	require.NoError(t, os.MkdirAll(settings.NginxSettings.ConfigDir, 0700))
	require.NoError(t, os.WriteFile(settings.NginxSettings.ConfigPath, []byte("events {}\nhttp {}\n"), 0600))
	require.NoError(t, os.WriteFile(cosysettings.ConfPath, []byte("[app]\nJwtSecret = "+cosysettings.AppSettings.JwtSecret+"\n[crypto]\nSecret = "+settings.CryptoSettings.Secret+"\n[database]\nName = backup-export\n"), 0600))
	// The database must be the actual on-disk file copied by Backup, not an
	// unrelated in-memory fixture. DELETE journaling puts committed sessions in it.
	database, err := gorm.Open(sqlite.Open(filepath.Join(root, "backup-export.db")+"?_journal_mode=DELETE"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.MCPServiceToken{}))
	model.Use(database)
	query.Use(database)
	query.SetDefault(database)
	admin := &model.User{Name: "backup-export-admin", Status: true, Language: "en"}
	if withOTP {
		admin.OTPSecret, err = crypto.AesEncrypt([]byte("JBSWY3DPEHPK3PXP"))
		require.NoError(t, err)
	}
	require.NoError(t, database.Create(admin).Error)
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	var session model.AuthToken
	require.NoError(t, database.Where("token = ?", payload.Token).First(&session).Error)
	_, writeToken, err := internalmcp.CreateServiceToken("backup-writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("backup-reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	router := gin.New()
	rootGroup := router.Group("/api")
	InitRouter(rootGroup)
	interactive := rootGroup.Group("", middleware.AuthRequired())
	userapi.InitUserRouter(interactive)
	mcp.InitManagementRouter(interactive)
	return backupExportFixture{router: router, admin: admin, database: database, jwt: payload.Token, shortToken: session.ShortToken, writeToken: writeToken, readToken: readToken}
}

func backupExportRequest(router http.Handler, method, path, token, secureSession string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if secureSession != "" {
		req.Header.Set("X-Secure-Session-ID", secureSession)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func backupZIPEntry(t *testing.T, content []byte, name string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	for _, entry := range reader.File {
		if entry.Name == name {
			stream, err := entry.Open()
			require.NoError(t, err)
			defer stream.Close()
			value, err := io.ReadAll(stream)
			require.NoError(t, err)
			return value
		}
	}
	t.Fatalf("backup ZIP missing %s", name)
	return nil
}

func assertBackupHasNoSessions(t *testing.T, fixture backupExportFixture, response *httptest.ResponseRecorder) {
	t.Helper()
	parts := strings.Split(response.Header().Get("X-Backup-Security"), ":")
	require.Len(t, parts, 2)
	key, err := base64.StdEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	iv, err := base64.StdEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	encrypted := backupZIPEntry(t, response.Body.Bytes(), internalbackup.NginxUIZipName)
	plaintext, err := internalbackup.AESDecrypt(encrypted, key, iv)
	require.NoError(t, err)
	exportedConfig := backupZIPEntry(t, plaintext, "app.ini")
	require.Contains(t, string(exportedConfig), "backup-export-synthetic-jwt-secret")
	dbPath := filepath.Join(t.TempDir(), "exported.db")
	databaseBytes := backupZIPEntry(t, plaintext, "backup-export.db")
	require.NotContains(t, string(databaseBytes), fixture.jwt)
	require.NotContains(t, string(databaseBytes), fixture.shortToken)
	require.NoError(t, os.WriteFile(dbPath, databaseBytes, 0600))
	exportedDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	exportedSQL, err := exportedDB.DB()
	require.NoError(t, err)
	defer exportedSQL.Close()
	var count int64
	require.NoError(t, exportedDB.Model(&model.AuthToken{}).Count(&count).Error)
	require.Zero(t, count)
	var account struct {
		Name   string
		Status bool
	}
	require.NoError(t, exportedDB.Table("users").Where("id = ?", fixture.admin.ID).First(&account).Error)
	require.Equal(t, fixture.admin.Name, account.Name)
	require.True(t, account.Status)
}

func TestBackupExportPreservesExistingAuthenticationRequirements(t *testing.T) {
	fixture := setupBackupExport(t, false)
	for _, tc := range []struct{ name, token string }{
		{"read PAT", fixture.readToken}, {"missing credential", ""}, {"invalid credential", "invalid-credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := backupExportRequest(fixture.router, http.MethodGet, "/api/backup", tc.token, "", nil)
			require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
			require.Empty(t, response.Header().Get("X-Backup-Security"))
			require.NotEqual(t, "application/zip", response.Header().Get("Content-Type"))
		})
	}
}

func TestBackupExportRemovesReusableSessionsAndPreservesMFAGates(t *testing.T) {
	for _, withOTP := range []bool{false, true} {
		t.Run(map[bool]string{false: "password-only", true: "MFA-enrolled"}[withOTP], func(t *testing.T) {
			fixture := setupBackupExport(t, withOTP)
			// A PAT must not acquire the active JWT used by the interactive-only
			// identity endpoints or token management. Backup is the same boundary.
			mintBody := []byte(`{"name":"interactive-backup-token","scopes":["api:write"]}`)
			for _, path := range []string{"/api/2fa_status", "/api/service_tokens"} {
				response := backupExportRequest(fixture.router, http.MethodGet, path, fixture.writeToken, "", nil)
				require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
			}
			response := backupExportRequest(fixture.router, http.MethodPost, "/api/service_tokens", fixture.writeToken, "", mintBody)
			require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())

			secureSession := ""
			if withOTP {
				blocked := backupExportRequest(fixture.router, http.MethodGet, "/api/backup", fixture.jwt, "", nil)
				require.Equal(t, http.StatusUnauthorized, blocked.Code, blocked.Body.String())
				require.Empty(t, blocked.Header().Get("X-Backup-Security"))
				secureSession = internaluser.SetSecureSessionID(fixture.admin.ID, fixture.admin.MFAVersion)
			}
			exported := backupExportRequest(fixture.router, http.MethodGet, "/api/backup", fixture.jwt, secureSession, nil)
			require.Equal(t, http.StatusOK, exported.Code, exported.Body.String())
			assertBackupHasNoSessions(t, fixture, exported)
			// Management credentials can still export persistent application data,
			// but no longer receive the bearer sessions needed for interactive APIs.
			exported = backupExportRequest(fixture.router, http.MethodGet, "/api/backup", fixture.writeToken, "", nil)
			require.Equal(t, http.StatusOK, exported.Code, exported.Body.String())
			assertBackupHasNoSessions(t, fixture, exported)
			request := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
			request.Header.Set("X-Node-Secret", settings.NodeSettings.Secret)
			nodeExport := httptest.NewRecorder()
			fixture.router.ServeHTTP(nodeExport, request)
			require.Equal(t, http.StatusOK, nodeExport.Code, nodeExport.Body.String())
			assertBackupHasNoSessions(t, fixture, nodeExport)
			identity := backupExportRequest(fixture.router, http.MethodGet, "/api/2fa_status", fixture.jwt, "", nil)
			require.Equal(t, http.StatusOK, identity.Code, identity.Body.String())
			var status userapi.Status2FA
			require.NoError(t, json.Unmarshal(identity.Body.Bytes(), &status))
			require.Equal(t, withOTP, status.OTPStatus)
			minted := backupExportRequest(fixture.router, http.MethodPost, "/api/service_tokens", fixture.jwt, "", mintBody)
			expected := http.StatusCreated
			if withOTP {
				expected = http.StatusUnauthorized
			}
			require.Equal(t, expected, minted.Code, minted.Body.String())
			// Live revocation remains effective without touching the source DB
			// during export. This test does not restore an old database snapshot.
			require.NoError(t, internaluser.RevokeSessionToken(fixture.jwt))
			replayed := backupExportRequest(fixture.router, http.MethodGet, "/api/2fa_status", fixture.jwt, "", nil)
			require.Equal(t, http.StatusForbidden, replayed.Code, replayed.Body.String())
		})
	}
}

func TestSessionFreeBackupRestoresAccountsWithoutRestoringLoginSessions(t *testing.T) {
	fixture := setupBackupExport(t, false)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("restore-test-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, fixture.database.Model(fixture.admin).Update("password", string(passwordHash)).Error)
	exported := backupExportRequest(fixture.router, http.MethodGet, "/api/backup", fixture.writeToken, "", nil)
	require.Equal(t, http.StatusOK, exported.Code, exported.Body.String())
	assertBackupHasNoSessions(t, fixture, exported)
	archive := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(archive, exported.Body.Bytes(), 0600))
	parts := strings.Split(exported.Header().Get("X-Backup-Security"), ":")
	require.Len(t, parts, 2)
	key, err := base64.StdEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	iv, err := base64.StdEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	liveSQL, err := fixture.database.DB()
	require.NoError(t, err)
	require.NoError(t, liveSQL.Close())
	result, err := internalbackup.Restore(internalbackup.RestoreOptions{
		BackupPath: archive, AESKey: key, AESIv: iv, RestoreDir: t.TempDir(),
		RestoreNginxUI: true, RestoreNginx: false, VerifyHash: true,
	})
	require.NoError(t, err)
	require.True(t, result.NginxUIRestored)
	require.True(t, result.HashMatch)
	restored, err := gorm.Open(sqlite.Open(filepath.Join(filepath.Dir(cosysettings.ConfPath), "backup-export.db")), &gorm.Config{})
	require.NoError(t, err)
	restoredSQL, err := restored.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = restoredSQL.Close() })
	model.Use(restored)
	query.Use(restored)
	query.SetDefault(restored)
	_, valid := internaluser.GetTokenUser(fixture.jwt)
	require.False(t, valid, "a restored backup must not restore its source JWT")
	_, valid = internaluser.GetTokenUserByShortToken(fixture.shortToken)
	require.False(t, valid, "a restored backup must not restore its source short token")
	account, err := internaluser.Login(fixture.admin.Name, "restore-test-password")
	require.NoError(t, err)
	login, err := internaluser.IssueLoginToken(account, internaluser.LoginProofPassword)
	require.NoError(t, err)
	require.NotEqual(t, fixture.jwt, login.Token)
	identity := backupExportRequest(fixture.router, http.MethodGet, "/api/2fa_status", login.Token, "", nil)
	require.Equal(t, http.StatusOK, identity.Code, identity.Body.String())
}
