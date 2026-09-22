package middleware_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	backupapi "github.com/0xJacky/Nginx-UI/api/backup"
	certificateapi "github.com/0xJacky/Nginx-UI/api/certificate"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
)

const (
	dnsCredentialSecret = "dns-api-token-secret"
	eabKeyIDSecret      = "eab-key-id-secret"
	eabHMACSecret       = "eab-hmac-secret"
	s3AccessKeySecret   = "s3-access-key-secret"
	s3SecretKeySecret   = "s3-secret-key-secret"
)

type sensitiveReadTestState struct {
	router           *gin.Engine
	serviceToken     string
	interactiveToken string
	dnsCredentialID  uint64
	acmeUserID       uint64
	autoBackupID     uint64
}

func setupSensitiveReadEndpoints(t *testing.T) sensitiveReadTestState {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalCryptoSecret := appsettings.CryptoSettings.Secret
	originalInstanceID := appsettings.NodeSettings.InstanceID
	originalJWTSecret := cosysettings.AppSettings.JwtSecret
	appsettings.CryptoSettings.Secret = "sensitive-read-endpoint-test-root"
	appsettings.NodeSettings.InstanceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	cosysettings.AppSettings.JwtSecret = "sensitive-read-jwt-secret"

	cosy.RegisterModels(
		&model.User{},
		&model.AuthToken{},
		&model.Passkey{},
		&model.MCPServiceToken{},
		&model.DnsCredential{},
		&model.AcmeUser{},
		&model.AutoBackup{},
	)
	database := cosy.InitDB(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())))
	model.Use(database)
	query.Use(database)
	query.SetDefault(database)

	interactiveUser := &model.User{Name: "interactive-admin", Status: true, Language: "en"}
	require.NoError(t, database.Create(interactiveUser).Error)
	interactivePayload, err := internaluser.GenerateJWT(interactiveUser)
	require.NoError(t, err)

	_, serviceToken, err := internalmcp.CreateServiceToken(
		"api-reader",
		[]string{model.APITokenScopeRead},
		nil,
		interactiveUser.ID,
	)
	require.NoError(t, err)

	dnsCredential := &model.DnsCredential{
		Name:         "Cloudflare DNS",
		Provider:     "Cloudflare",
		ProviderCode: "cloudflare",
		Config: &dns.Config{
			Name: "Cloudflare",
			Code: "cloudflare",
			Configuration: &dns.Configuration{
				Credentials: map[string]string{"CF_DNS_API_TOKEN": dnsCredentialSecret},
				Additional:  map[string]string{"CF_DNS_HTTP_TIMEOUT": "30"},
			},
		},
	}
	require.NoError(t, database.Create(dnsCredential).Error)

	acmeUser := &model.AcmeUser{
		Name:       "ACME account",
		Email:      "admin@example.com",
		CADir:      "https://ca.example.com/directory",
		EABKeyID:   eabKeyIDSecret,
		EABHMACKey: eabHMACSecret,
	}
	require.NoError(t, database.Create(acmeUser).Error)

	autoBackup := &model.AutoBackup{
		Name:              "S3 backup",
		BackupType:        model.BackupTypeNginxAndNginxUI,
		StorageType:       model.StorageTypeS3,
		StoragePath:       "backups/",
		CronExpression:    "0 0 * * *",
		Enabled:           true,
		LastBackupStatus:  model.BackupStatusPending,
		S3Endpoint:        "https://s3.example.com",
		S3AccessKeyID:     s3AccessKeySecret,
		S3SecretAccessKey: s3SecretKeySecret,
		S3Bucket:          "nginx-ui",
		S3Region:          "us-east-1",
	}
	require.NoError(t, database.Create(autoBackup).Error)

	router := gin.New()
	group := router.Group("/api", middleware.AuthRequired())
	certificateapi.InitDNSCredentialRouter(group)
	certificateapi.InitAcmeUserRouter(group)
	backupapi.InitAutoBackupRouter(group)

	t.Cleanup(func() {
		cache.Shutdown()
		model.Use(nil)
		appsettings.CryptoSettings.Secret = originalCryptoSecret
		appsettings.NodeSettings.InstanceID = originalInstanceID
		cosysettings.AppSettings.JwtSecret = originalJWTSecret
	})

	return sensitiveReadTestState{
		router:           router,
		serviceToken:     serviceToken,
		interactiveToken: interactivePayload.Token,
		dnsCredentialID:  dnsCredential.ID,
		acmeUserID:       acmeUser.ID,
		autoBackupID:     autoBackup.ID,
	}
}

func getSensitiveReadEndpoint(t *testing.T, router http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	return recorder
}

func responseItem(t *testing.T, recorder *httptest.ResponseRecorder, isList bool) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	if !isList {
		return response
	}

	data, ok := response["data"].([]any)
	require.True(t, ok, recorder.Body.String())
	require.Len(t, data, 1)
	item, ok := data[0].(map[string]any)
	require.True(t, ok, recorder.Body.String())
	return item
}

func TestAPIReadServiceTokenRedactsStoredCredentials(t *testing.T) {
	state := setupSensitiveReadEndpoints(t)

	tests := []struct {
		name            string
		path            string
		isList          bool
		redactedFields  []string
		secretFragments []string
	}{
		{
			name:            "DNS credential list",
			path:            "/api/dns_credentials",
			isList:          true,
			redactedFields:  []string{"config"},
			secretFragments: []string{dnsCredentialSecret},
		},
		{
			name:            "DNS credential detail",
			path:            "/api/dns_credentials/" + strconv.FormatUint(state.dnsCredentialID, 10),
			redactedFields:  []string{"configuration", "links"},
			secretFragments: []string{dnsCredentialSecret},
		},
		{
			name:            "ACME user list",
			path:            "/api/acme_users",
			isList:          true,
			redactedFields:  []string{"eab_key_id", "eab_hmac_key"},
			secretFragments: []string{eabKeyIDSecret, eabHMACSecret},
		},
		{
			name:            "ACME user detail",
			path:            "/api/acme_users/" + strconv.FormatUint(state.acmeUserID, 10),
			redactedFields:  []string{"eab_key_id", "eab_hmac_key"},
			secretFragments: []string{eabKeyIDSecret, eabHMACSecret},
		},
		{
			name:            "auto backup list",
			path:            "/api/auto_backup",
			isList:          true,
			redactedFields:  []string{"s3_access_key_id", "s3_secret_access_key"},
			secretFragments: []string{s3AccessKeySecret, s3SecretKeySecret},
		},
		{
			name:            "auto backup detail",
			path:            "/api/auto_backup/" + strconv.FormatUint(state.autoBackupID, 10),
			redactedFields:  []string{"s3_access_key_id", "s3_secret_access_key"},
			secretFragments: []string{s3AccessKeySecret, s3SecretKeySecret},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := getSensitiveReadEndpoint(t, state.router, test.path, state.serviceToken)
			item := responseItem(t, recorder, test.isList)
			require.NotEmpty(t, item["name"])
			for _, field := range test.redactedFields {
				_, exposed := item[field]
				require.Falsef(t, exposed, "field %q must be absent: %s", field, recorder.Body.String())
			}
			for _, secret := range test.secretFragments {
				require.False(t, strings.Contains(recorder.Body.String(), secret), recorder.Body.String())
			}
		})
	}
}

func TestInteractiveAdministratorKeepsStoredCredentialResponses(t *testing.T) {
	state := setupSensitiveReadEndpoints(t)

	tests := []struct {
		name            string
		path            string
		isList          bool
		expectedFields  []string
		secretFragments []string
	}{
		{
			name:            "DNS credential list",
			path:            "/api/dns_credentials",
			isList:          true,
			expectedFields:  []string{"config"},
			secretFragments: []string{dnsCredentialSecret},
		},
		{
			name:            "DNS credential detail",
			path:            "/api/dns_credentials/" + strconv.FormatUint(state.dnsCredentialID, 10),
			expectedFields:  []string{"configuration"},
			secretFragments: []string{dnsCredentialSecret},
		},
		{
			name:            "ACME user list",
			path:            "/api/acme_users",
			isList:          true,
			expectedFields:  []string{"eab_key_id", "eab_hmac_key"},
			secretFragments: []string{eabKeyIDSecret, eabHMACSecret},
		},
		{
			name:            "ACME user detail",
			path:            "/api/acme_users/" + strconv.FormatUint(state.acmeUserID, 10),
			expectedFields:  []string{"eab_key_id", "eab_hmac_key"},
			secretFragments: []string{eabKeyIDSecret, eabHMACSecret},
		},
		{
			name:            "auto backup list",
			path:            "/api/auto_backup",
			isList:          true,
			expectedFields:  []string{"s3_access_key_id", "s3_secret_access_key"},
			secretFragments: []string{s3AccessKeySecret, s3SecretKeySecret},
		},
		{
			name:            "auto backup detail",
			path:            "/api/auto_backup/" + strconv.FormatUint(state.autoBackupID, 10),
			expectedFields:  []string{"s3_access_key_id", "s3_secret_access_key"},
			secretFragments: []string{s3AccessKeySecret, s3SecretKeySecret},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := getSensitiveReadEndpoint(t, state.router, test.path, state.interactiveToken)
			item := responseItem(t, recorder, test.isList)
			for _, field := range test.expectedFields {
				_, present := item[field]
				require.Truef(t, present, "field %q must remain present: %s", field, recorder.Body.String())
			}
			for _, secret := range test.secretFragments {
				require.True(t, strings.Contains(recorder.Body.String(), secret), recorder.Body.String())
			}
		})
	}
}
