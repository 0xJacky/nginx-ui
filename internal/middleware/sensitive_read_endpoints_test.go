package middleware_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	backupapi "github.com/0xJacky/Nginx-UI/api/backup"
	certificateapi "github.com/0xJacky/Nginx-UI/api/certificate"
	configapi "github.com/0xJacky/Nginx-UI/api/config"
	dnsapi "github.com/0xJacky/Nginx-UI/api/dns"
	externalnotifyapi "github.com/0xJacky/Nginx-UI/api/external_notify"
	sitesapi "github.com/0xJacky/Nginx-UI/api/sites"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
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
	database := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
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

const (
	notifierSecret      = "notify-bot-token-secret"
	healthCheckSecret   = "Bearer health-check-secret"
	dnsAdditionalSecret = "dns-additional-session-token"
	privateKeyMarker    = "PRIVATE KEY"
)

// storedSecretReadState serves the same API both directly and through a
// controller that proxies to it as a child node. The child only sees the
// controller's node principal, so the proxied variant proves the controller
// redacts on behalf of the original read-only caller.
type storedSecretReadState struct {
	local        http.Handler
	controller   http.Handler
	nodeID       uint64
	readToken    string
	writeToken   string
	adminToken   string
	certPEM      string
	keyPEM       string
	keyPath      string
	notifierID   uint64
	siteConfigID uint64
	certID       uint64
	domainID     uint64
}

type storedSecretTarget struct {
	name   string
	router func(storedSecretReadState) http.Handler
	nodeID func(storedSecretReadState) uint64
}

var storedSecretTargets = []storedSecretTarget{
	{
		name:   "local",
		router: func(state storedSecretReadState) http.Handler { return state.local },
		nodeID: func(storedSecretReadState) uint64 { return 0 },
	},
	{
		name:   "proxied",
		router: func(state storedSecretReadState) http.Handler { return state.controller },
		nodeID: func(state storedSecretReadState) uint64 { return state.nodeID },
	},
}

func newStoredSecretTestPair(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		DNSNames:     []string{"example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func setupStoredSecretReadEndpoints(t *testing.T) storedSecretReadState {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	previousNginx, previousNode := *appsettings.NginxSettings, *appsettings.NodeSettings
	previousCrypto := appsettings.CryptoSettings.Secret
	previousJWT := cosysettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		cache.Shutdown()
		model.Use(nil)
		*appsettings.NginxSettings, *appsettings.NodeSettings = previousNginx, previousNode
		appsettings.CryptoSettings.Secret = previousCrypto
		cosysettings.AppSettings.JwtSecret = previousJWT
	})
	appsettings.CryptoSettings.Secret = "stored-secret-read-test-root"
	cosysettings.AppSettings.JwtSecret = "stored-secret-read-jwt-secret"
	appsettings.NodeSettings.InstanceID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	appsettings.NodeSettings.Secret = "stored-secret-read-child-secret"
	appsettings.NodeSettings.Demo = false

	root := t.TempDir()
	appsettings.NginxSettings.ConfigDir = root
	appsettings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	appsettings.NginxSettings.HostMode = ""
	appsettings.NginxSettings.ContainerName = ""
	confDir := nginx.GetConfPath()
	sslDir := filepath.Join(confDir, "ssl")
	require.NoError(t, os.MkdirAll(sslDir, 0o700))
	certPEM, keyPEM := newStoredSecretTestPair(t)
	nginxConfPath := filepath.Join(confDir, "nginx.conf")
	certPath := filepath.Join(sslDir, "example.crt")
	keyPath := filepath.Join(sslDir, "example.key")
	for path, content := range map[string][]byte{
		nginxConfPath:                       []byte("events {}\nhttp {}\n"),
		certPath:                            certPEM,
		keyPath:                             keyPEM,
		filepath.Join(sslDir, "bundle.pem"): append(append([]byte{}, certPEM...), keyPEM...),
	} {
		require.NoError(t, os.WriteFile(path, content, 0o600))
	}

	cosy.RegisterModels(
		&model.User{},
		&model.AuthToken{},
		&model.Passkey{},
		&model.MCPServiceToken{},
		&model.Node{},
		&model.ExternalNotify{},
		&model.SiteConfig{},
		&model.Cert{},
		&model.Config{},
		&model.ConfigBackup{},
		&model.DnsCredential{},
		&model.DnsDomain{},
	)
	database := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
	model.Use(database)
	query.Use(database)
	query.SetDefault(database)

	admin := &model.User{Name: "stored-secret-admin", Status: true, Language: "en"}
	require.NoError(t, database.Create(admin).Error)
	adminPayload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("api-reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("api-writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)

	notifier := &model.ExternalNotify{
		Type:        "telegram",
		Description: "On-call",
		Language:    "en",
		Config:      map[string]string{"bot_token": notifierSecret, "chat_id": "42"},
		Enabled:     true,
	}
	require.NoError(t, database.Create(notifier).Error)

	siteConfig := &model.SiteConfig{
		SiteKey:            "example",
		SiteName:           "example",
		Host:               "example.com:443",
		Port:               443,
		Scheme:             "https",
		HealthCheckEnabled: true,
		HealthCheckConfig: &model.HealthCheckConfig{
			Protocol: "https",
			Method:   "GET",
			Path:     "/health",
			Headers:  map[string]string{"Authorization": healthCheckSecret},
		},
	}
	require.NoError(t, database.Create(siteConfig).Error)

	certificate := &model.Cert{Name: "example", SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
	require.NoError(t, database.Create(certificate).Error)

	require.NoError(t, database.Create(&model.ConfigBackup{
		Name:     "example.key",
		FilePath: keyPath,
		Content:  string(keyPEM),
	}).Error)
	require.NoError(t, database.Create(&model.ConfigBackup{
		Name:     "nginx.conf",
		FilePath: nginxConfPath,
		Content:  "events {}\n# previous revision\n",
	}).Error)

	credential := &model.DnsCredential{
		Name:         "Cloudflare DNS",
		Provider:     "Cloudflare",
		ProviderCode: "cloudflare",
		Config: &dns.Config{
			Name: "Cloudflare",
			Code: "cloudflare",
			Configuration: &dns.Configuration{
				Credentials: map[string]string{"CF_DNS_API_TOKEN": dnsCredentialSecret},
				Additional:  map[string]string{"CF_SESSION_TOKEN": dnsAdditionalSecret},
			},
		},
	}
	require.NoError(t, database.Create(credential).Error)
	domain := &model.DnsDomain{Domain: "example.com", Description: "Primary", DnsCredentialID: credential.ID}
	require.NoError(t, database.Create(domain).Error)

	register := func(group *gin.RouterGroup) {
		certificateapi.InitCertificateRouter(group)
		configapi.InitRouter(group)
		dnsapi.InitRouter(group)
		externalnotifyapi.InitRouter(group)
		sitesapi.InitRouter(group)
	}
	local := gin.New()
	register(local.Group("/api", middleware.AuthRequired()))
	childServer := httptest.NewServer(local)
	t.Cleanup(childServer.Close)

	node := &model.Node{Name: "stored-secret-child", URL: childServer.URL, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, database.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(
		nodeauth.LegacyCredentialPurpose(node.ID),
		[]byte(appsettings.NodeSettings.Secret),
	)
	require.NoError(t, err)
	require.NoError(t, database.Model(node).Update("encrypted_legacy_secret", encrypted).Error)

	controller := gin.New()
	register(controller.Group("/api", middleware.AuthRequired(), middleware.Proxy()))

	return storedSecretReadState{
		local:        local,
		controller:   controller,
		nodeID:       node.ID,
		readToken:    readToken,
		writeToken:   writeToken,
		adminToken:   adminPayload.Token,
		certPEM:      string(certPEM),
		keyPEM:       string(keyPEM),
		keyPath:      keyPath,
		notifierID:   notifier.ID,
		siteConfigID: siteConfig.ID,
		certID:       certificate.ID,
		domainID:     domain.ID,
	}
}

func storedSecretRequest(router http.Handler, path, token string, nodeID uint64) *httptest.ResponseRecorder {
	// A cancelable context keeps httputil.ReverseProxy from falling back to
	// CloseNotify, which the response recorder does not implement.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+token)
	if nodeID != 0 {
		request.Header.Set("X-Node-ID", strconv.FormatUint(nodeID, 10))
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	return body
}

// jsonAt walks decoded JSON by object keys and array indexes.
func jsonAt(t *testing.T, value any, path ...string) any {
	t.Helper()
	for _, key := range path {
		switch typed := value.(type) {
		case map[string]any:
			child, ok := typed[key]
			require.Truef(t, ok, "missing key %q in %v", key, typed)
			value = child
		case []any:
			index, err := strconv.Atoi(key)
			require.NoError(t, err)
			require.Less(t, index, len(typed))
			value = typed[index]
		default:
			t.Fatalf("cannot descend into %T at %q", value, key)
		}
	}
	return value
}

func configBackupContent(t *testing.T, body map[string]any, name string) string {
	t.Helper()
	items, ok := body["data"].([]any)
	require.True(t, ok, body)
	for _, item := range items {
		backup := item.(map[string]any)
		if backup["name"] == name {
			return backup["content"].(string)
		}
	}
	t.Fatalf("config backup %q not found in %v", name, body)
	return ""
}

func configPath(path string) string {
	return "/api/config?path=" + url.QueryEscape(path)
}

func TestAPIReadServiceTokenRedactsEveryStoredSecret(t *testing.T) {
	state := setupStoredSecretReadEndpoints(t)
	placeholder := appsettings.RedactedSensitiveValue
	secrets := []string{notifierSecret, healthCheckSecret, dnsCredentialSecret, dnsAdditionalSecret, privateKeyMarker}

	tests := []struct {
		name  string
		path  string
		check func(t *testing.T, body map[string]any)
	}{
		{
			name: "notifier list",
			path: "/api/external_notifies",
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, "telegram", jsonAt(t, body, "data", "0", "type"))
				require.Equal(t, "On-call", jsonAt(t, body, "data", "0", "description"))
				require.Equal(t, placeholder, jsonAt(t, body, "data", "0", "config", "bot_token"))
				require.Equal(t, placeholder, jsonAt(t, body, "data", "0", "config", "chat_id"))
			},
		},
		{
			name: "notifier detail",
			path: "/api/external_notifies/" + strconv.FormatUint(state.notifierID, 10),
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, "telegram", jsonAt(t, body, "type"))
				require.Equal(t, placeholder, jsonAt(t, body, "config", "bot_token"))
			},
		},
		{
			name: "site health check",
			path: "/api/site_navigation/health_check/" + strconv.FormatUint(state.siteConfigID, 10),
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, "example.com:443", jsonAt(t, body, "host"))
				require.Equal(t, "/health", jsonAt(t, body, "health_check_config", "path"))
				require.Equal(t, placeholder, jsonAt(t, body, "health_check_config", "headers", "Authorization"))
			},
		},
		{
			name: "certificate detail",
			path: "/api/certs/" + strconv.FormatUint(state.certID, 10),
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, state.certPEM, jsonAt(t, body, "ssl_certificate"))
				require.Equal(t, state.keyPath, jsonAt(t, body, "ssl_certificate_key_path"))
				require.Equal(t, placeholder, jsonAt(t, body, "ssl_certificate_key"))
			},
		},
		{
			name: "config history",
			path: "/api/config_histories",
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, placeholder, configBackupContent(t, body, "example.key"))
				require.Equal(t, "events {}\n# previous revision\n", configBackupContent(t, body, "nginx.conf"))
			},
		},
		{
			name: "DNS domain list",
			path: "/api/dns/domains",
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, "example.com", jsonAt(t, body, "data", "0", "domain"))
				require.Equal(t, "Cloudflare DNS", jsonAt(t, body, "data", "0", "dns_credential", "name"))
				configuration := jsonAt(t, body, "data", "0", "dns_credential", "config", "configuration")
				require.Equal(t, placeholder, jsonAt(t, configuration, "credentials", "CF_DNS_API_TOKEN"))
				require.Equal(t, placeholder, jsonAt(t, configuration, "additional", "CF_SESSION_TOKEN"))
			},
		},
		{
			name: "DNS domain detail",
			path: "/api/dns/domains/" + strconv.FormatUint(state.domainID, 10),
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, "example.com", jsonAt(t, body, "domain"))
				configuration := jsonAt(t, body, "dns_credential", "config", "configuration")
				require.Equal(t, placeholder, jsonAt(t, configuration, "credentials", "CF_DNS_API_TOKEN"))
				require.Equal(t, placeholder, jsonAt(t, configuration, "additional", "CF_SESSION_TOKEN"))
			},
		},
		{
			name: "non-key data file",
			path: configPath("ssl/example.crt"),
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, state.certPEM, jsonAt(t, body, "content"))
			},
		},
		{
			name: "nginx configuration file",
			path: configPath("nginx.conf"),
			check: func(t *testing.T, body map[string]any) {
				require.Equal(t, "events {}\nhttp {}\n", jsonAt(t, body, "content"))
			},
		},
	}

	for _, target := range storedSecretTargets {
		t.Run(target.name, func(t *testing.T) {
			router, nodeID := target.router(state), target.nodeID(state)
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					recorder := storedSecretRequest(router, test.path, state.readToken, nodeID)
					require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					for _, secret := range secrets {
						require.NotContains(t, recorder.Body.String(), secret)
					}
					test.check(t, decodeJSONBody(t, recorder))
				})
			}

			for _, keyFile := range []struct{ name, path string }{
				{"key extension", "ssl/example.key"},
				{"PEM key in a bundle", "ssl/bundle.pem"},
				{"absolute key path", state.keyPath},
			} {
				t.Run("private key file/"+keyFile.name, func(t *testing.T) {
					recorder := storedSecretRequest(router, configPath(keyFile.path), state.readToken, nodeID)
					require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
					require.NotContains(t, recorder.Body.String(), privateKeyMarker)
					body := decodeJSONBody(t, recorder)
					require.Equal(t, "config", body["scope"])
					require.EqualValues(t, 40019, body["code"])
				})
			}
		})
	}
}

func TestManagementTokensAndAdministratorsKeepStoredSecrets(t *testing.T) {
	state := setupStoredSecretReadEndpoints(t)

	tests := []struct {
		name    string
		path    string
		secrets []string
	}{
		{"notifier list", "/api/external_notifies", []string{notifierSecret}},
		{"notifier detail", "/api/external_notifies/" + strconv.FormatUint(state.notifierID, 10), []string{notifierSecret}},
		{"site health check", "/api/site_navigation/health_check/" + strconv.FormatUint(state.siteConfigID, 10), []string{healthCheckSecret}},
		{"certificate detail", "/api/certs/" + strconv.FormatUint(state.certID, 10), []string{privateKeyMarker}},
		{"config history", "/api/config_histories", []string{privateKeyMarker}},
		{"DNS domain list", "/api/dns/domains", []string{dnsCredentialSecret, dnsAdditionalSecret}},
		{"DNS domain detail", "/api/dns/domains/" + strconv.FormatUint(state.domainID, 10), []string{dnsCredentialSecret}},
		{"private key file", configPath("ssl/example.key"), []string{privateKeyMarker}},
		{"combined key bundle", configPath("ssl/bundle.pem"), []string{privateKeyMarker}},
	}

	for _, target := range storedSecretTargets {
		for _, caller := range []struct {
			name  string
			token string
		}{
			{"api:write token", state.writeToken},
			{"interactive administrator", state.adminToken},
		} {
			t.Run(target.name+"/"+caller.name, func(t *testing.T) {
				router, nodeID := target.router(state), target.nodeID(state)
				for _, test := range tests {
					t.Run(test.name, func(t *testing.T) {
						recorder := storedSecretRequest(router, test.path, caller.token, nodeID)
						require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
						require.NotContains(t, recorder.Body.String(), appsettings.RedactedSensitiveValue)
						for _, secret := range test.secrets {
							require.Contains(t, recorder.Body.String(), secret)
						}
					})
				}

				recorder := storedSecretRequest(router, configPath("ssl/example.key"), caller.token, nodeID)
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, state.keyPEM, decodeJSONBody(t, recorder)["content"])
			})
		}
	}
}

// The site navigation feed streams health check headers, and WebSocket frames
// cannot be rewritten while the controller relays them, so a read-only token
// may not open a child node's feed through the controller.
func TestAPIReadServiceTokenCannotRelayRemoteSiteNavigationFeed(t *testing.T) {
	state := setupStoredSecretReadEndpoints(t)
	router := gin.New()
	ws := router.Group("/api", middleware.AuthRequiredWS(), middleware.ProxyWs())
	ws.GET("site_navigation_ws", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := storedSecretRequest(router, "/api/site_navigation_ws", state.readToken, state.nodeID)
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	body := decodeJSONBody(t, recorder)
	require.Equal(t, "middleware", body["scope"])
	require.EqualValues(t, 40301, body["code"])

	// The controller's own feed redacts in the handler, so it stays available.
	recorder = storedSecretRequest(router, "/api/site_navigation_ws", state.readToken, 0)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())

	for _, token := range []string{state.writeToken, state.adminToken} {
		recorder = storedSecretRequest(router, "/api/site_navigation_ws", token, state.nodeID)
		require.NotEqual(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}
