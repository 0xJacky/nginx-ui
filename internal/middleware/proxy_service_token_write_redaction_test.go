package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	backupapi "github.com/0xJacky/Nginx-UI/api/backup"
	certificateapi "github.com/0xJacky/Nginx-UI/api/certificate"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// relayedWriteFixture is a controller that relays to a child node on a
// loopback listener. Both engines share one database, so the child serves and
// changes the same rows the test inspects.
type relayedWriteFixture struct {
	db         *gorm.DB
	controller *gin.Engine
	nodeID     uint64
	readToken  string
	writeToken string
	adminToken string
}

func setupRelayedWriteFixture(t *testing.T) relayedWriteFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger.Init("fatal")
	cache.InitInMemoryCache()
	oldCrypto := appsettings.CryptoSettings.Secret
	oldInstance := appsettings.NodeSettings.InstanceID
	oldSecret := appsettings.NodeSettings.Secret
	oldJWT := cosysettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		appsettings.CryptoSettings.Secret = oldCrypto
		appsettings.NodeSettings.InstanceID = oldInstance
		appsettings.NodeSettings.Secret = oldSecret
		cosysettings.AppSettings.JwtSecret = oldJWT
		model.Use(nil)
	})
	appsettings.CryptoSettings.Secret = "relayed-write-redaction-crypto-root"
	appsettings.NodeSettings.InstanceID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	appsettings.NodeSettings.Secret = "relayed-write-redaction-node-secret"
	cosysettings.AppSettings.JwtSecret = "relayed-write-redaction-jwt"

	cosy.RegisterModels(&model.User{}, &model.Passkey{}, &model.AuthToken{}, &model.MCPServiceToken{}, &model.Node{},
		&model.AutoBackup{}, &model.DnsCredential{}, &model.AcmeUser{})
	db := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	// No second factor is enrolled, so the administrator needs no secure
	// session for the writes below.
	admin := &model.User{Name: "relayed-write-admin", Status: true}
	require.NoError(t, db.Create(admin).Error)
	t.Cleanup(func() { internaluser.InvalidateUserCache(admin.ID) })
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)

	register := func(g *gin.RouterGroup) {
		backupapi.InitAutoBackupRouter(g)
		certificateapi.InitDNSCredentialRouter(g)
		certificateapi.InitAcmeUserRouter(g)
	}

	child := gin.New()
	register(child.Group("/api", middleware.AuthRequired()))
	childServer := httptest.NewServer(child)
	t.Cleanup(childServer.Close)

	node := &model.Node{Name: "child", URL: childServer.URL, Enabled: true, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, db.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte(appsettings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", encrypted).Error)

	controller := gin.New()
	register(controller.Group("/api", middleware.AuthRequired(), middleware.Proxy()))

	return relayedWriteFixture{
		db: db, controller: controller, nodeID: node.ID,
		readToken: readToken, writeToken: writeToken, adminToken: payload.Token,
	}
}

// relay sends a request through the controller to the child node.
func (f relayedWriteFixture) relay(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(method, path, reader).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Node-ID", strconv.FormatUint(f.nodeID, 10))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	f.controller.ServeHTTP(res, req)
	return res
}

func recordID(t *testing.T, body string) string {
	t.Helper()
	var record struct {
		ID uint64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &record), body)
	require.NotZero(t, record.ID, body)
	return strconv.FormatUint(record.ID, 10)
}

// requireRedacted checks one relayed answer: it keeps the record it is about
// and, for a service token, carries none of the secrets or their fields.
func requireRedacted(t *testing.T, body, keep string, seesSecrets bool, secrets, fields []string) {
	t.Helper()
	require.Contains(t, body, keep, "the rest of the record is still returned")
	for _, secret := range secrets {
		require.Equal(t, seesSecrets, strings.Contains(body, secret), body)
	}
	if seesSecrets {
		return
	}
	for _, field := range fields {
		require.NotContains(t, body, `"`+field+`"`, body)
	}
}

type relayedWriteCaller struct {
	name        string
	slug        string
	token       func(relayedWriteFixture) string
	seesSecrets bool
}

var relayedWriteCallers = []relayedWriteCaller{
	{"api:write", "writer", func(f relayedWriteFixture) string { return f.writeToken }, false},
	{"interactive administrator", "admin", func(f relayedWriteFixture) string { return f.adminToken }, true},
}

// A child node answers the controller's node principal in full, so the
// controller removes stored credentials from every answer it relays to a
// service token. Writes answer in other shapes than list reads: a create or
// modify answers the record itself, a delete or recover answers 204 without a
// body. Each shape must be relayed with the child's status, after the child
// did the work, and still without the credentials. Interactive administrators
// keep receiving them.
func TestRelayedServiceTokenWritesKeepRedactionInEveryShape(t *testing.T) {
	f := setupRelayedWriteFixture(t)

	t.Run("auto backup", func(t *testing.T) {
		fields := []string{"s3_access_key_id", "s3_secret_access_key", "storage_config"}
		for _, caller := range relayedWriteCallers {
			t.Run(caller.name, func(t *testing.T) {
				token := caller.token(f)
				prefix := caller.slug
				accessKey, secretKey, storageSecret := prefix+"-AK-1", prefix+"-SK-1", prefix+"-STORAGE-1"
				name := prefix + "-backup"

				res := f.relay(t, http.MethodPost, "/api/auto_backup", token, map[string]any{
					"name": name, "backup_type": model.BackupTypeNginxAndNginxUI, "storage_type": model.StorageTypeS3,
					"storage_path": "nginx-ui", "cron_expression": "0 0 * * *", "s3_bucket": "bucket",
					"s3_access_key_id": accessKey, "s3_secret_access_key": secretKey,
					"storage_config": map[string]string{"secret_key": storageSecret},
				})
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name, caller.seesSecrets, []string{accessKey, secretKey, storageSecret}, fields)
				id := recordID(t, res.Body.String())
				var stored model.AutoBackup
				require.NoError(t, f.db.First(&stored, id).Error, "the child created the task")
				require.Equal(t, secretKey, stored.S3SecretAccessKey)

				rotated := prefix + "-SK-2"
				res = f.relay(t, http.MethodPost, "/api/auto_backup/"+id, token, map[string]any{
					"name": name + "-renamed", "s3_secret_access_key": rotated,
				})
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{accessKey, rotated, storageSecret}, fields)
				require.NoError(t, f.db.First(&stored, id).Error)
				require.Equal(t, rotated, stored.S3SecretAccessKey, "the child modified the task")

				res = f.relay(t, http.MethodGet, "/api/auto_backup/"+id, token, nil)
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{accessKey, rotated, storageSecret}, fields)

				res = f.relay(t, http.MethodGet, "/api/auto_backup", token, nil)
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{accessKey, rotated, storageSecret}, fields)

				res = f.relay(t, http.MethodDelete, "/api/auto_backup/"+id, token, nil)
				require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
				require.Empty(t, res.Body.String())
				require.Error(t, f.db.First(&model.AutoBackup{}, id).Error, "the child deleted the task")
			})
		}
	})

	t.Run("dns credential", func(t *testing.T) {
		fields := []string{"config", "configuration", "links"}
		for _, caller := range relayedWriteCallers {
			t.Run(caller.name, func(t *testing.T) {
				token := caller.token(f)
				prefix := caller.slug
				secret, name := prefix+"-DNS-1", prefix+"-credential"
				credential := func(name, secret string) map[string]any {
					return map[string]any{
						"name": name, "provider": "Cloudflare", "provider_code": "cloudflare",
						"configuration": map[string]any{"credentials": map[string]string{"CF_DNS_API_TOKEN": secret}},
					}
				}

				res := f.relay(t, http.MethodPost, "/api/dns_credentials", token, credential(name, secret))
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name, caller.seesSecrets, []string{secret}, fields)
				id := recordID(t, res.Body.String())
				require.NoError(t, f.db.First(&model.DnsCredential{}, id).Error, "the child created the credential")

				rotated := prefix + "-DNS-2"
				res = f.relay(t, http.MethodPost, "/api/dns_credentials/"+id, token, credential(name+"-renamed", rotated))
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{rotated}, fields)
				var stored model.DnsCredential
				require.NoError(t, f.db.First(&stored, id).Error)
				require.Equal(t, rotated, stored.Config.Configuration.Credentials["CF_DNS_API_TOKEN"], "the child modified the credential")

				res = f.relay(t, http.MethodGet, "/api/dns_credentials", token, nil)
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{rotated}, fields)

				res = f.relay(t, http.MethodDelete, "/api/dns_credentials/"+id, token, nil)
				require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
				require.Empty(t, res.Body.String())
				require.Error(t, f.db.First(&model.DnsCredential{}, id).Error, "the child deleted the credential")
			})
		}
	})

	t.Run("acme user", func(t *testing.T) {
		fields := []string{"eab_key_id", "eab_hmac_key"}
		for _, caller := range relayedWriteCallers {
			t.Run(caller.name, func(t *testing.T) {
				token := caller.token(f)
				prefix := caller.slug
				keyID, hmacKey, name := prefix+"-EAB-ID", prefix+"-EAB-HMAC", prefix+"-acme"
				// Seeded directly: creating one registers it with a CA.
				user := &model.AcmeUser{Name: name, Email: prefix + "@example.com", CADir: "https://acme.invalid/directory",
					EABKeyID: keyID, EABHMACKey: hmacKey}
				require.NoError(t, f.db.Create(user).Error)
				id := strconv.FormatUint(user.ID, 10)

				// The email and directory stay the same, so the child does
				// not register the account again.
				res := f.relay(t, http.MethodPost, "/api/acme_users/"+id, token, map[string]any{
					"name": name + "-renamed", "email": user.Email, "ca_dir": user.CADir,
				})
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{keyID, hmacKey}, fields)

				res = f.relay(t, http.MethodGet, "/api/acme_users/"+id, token, nil)
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{keyID, hmacKey}, fields)

				res = f.relay(t, http.MethodGet, "/api/acme_users", token, nil)
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				requireRedacted(t, res.Body.String(), name+"-renamed", caller.seesSecrets, []string{keyID, hmacKey}, fields)

				res = f.relay(t, http.MethodDelete, "/api/acme_users/"+id, token, nil)
				require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
				require.Empty(t, res.Body.String())
				require.Error(t, f.db.First(&model.AcmeUser{}, id).Error, "the child deleted the account")

				res = f.relay(t, http.MethodPatch, "/api/acme_users/"+id, token, nil)
				require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
				require.Empty(t, res.Body.String())
				require.NoError(t, f.db.First(&model.AcmeUser{}, id).Error, "the child recovered the account")
			})
		}
	})

	// A read-only token still reads the relayed records without credentials.
	t.Run("api:read reads", func(t *testing.T) {
		const secret = "READER-SK"
		task := &model.AutoBackup{Name: "reader-backup", BackupType: model.BackupTypeNginxAndNginxUI, StorageType: model.StorageTypeS3,
			StoragePath: "nginx-ui", CronExpression: "0 0 * * *", S3Bucket: "bucket", S3AccessKeyID: "READER-AK", S3SecretAccessKey: secret}
		require.NoError(t, f.db.Create(task).Error)
		for _, path := range []string{"/api/auto_backup", "/api/auto_backup/" + strconv.FormatUint(task.ID, 10)} {
			res := f.relay(t, http.MethodGet, path, f.readToken, nil)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())
			requireRedacted(t, res.Body.String(), task.Name, false, []string{secret, "READER-AK"},
				[]string{"s3_access_key_id", "s3_secret_access_key", "storage_config"})
		}
	})
}
