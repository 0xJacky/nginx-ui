package settings

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uozi-tech/cosy/logger"
)

func TestDeprecatedEnvMigration(t *testing.T) {
	logger.Init("debug")
	// Deprecated
	_ = os.Setenv("NGINX_UI_SERVER_HTTP_HOST", "127.0.0.1")
	_ = os.Setenv("NGINX_UI_SERVER_HTTP_PORT", "8080")
	// _ = os.Setenv("NGINX_UI_SERVER_RUN_MODE", "testing")
	_ = os.Setenv("NGINX_UI_SERVER_JWT_SECRET", "newSecret123")
	_ = os.Setenv("NGINX_UI_SERVER_NODE_SECRET", "newSercet123")
	_ = os.Setenv("NGINX_UI_SERVER_HTTP_CHALLENGE_PORT", "9181")
	_ = os.Setenv("NGINX_UI_SERVER_EMAIL", "test")
	_ = os.Setenv("NGINX_UI_SERVER_DATABASE", "testDB")
	_ = os.Setenv("NGINX_UI_SERVER_START_CMD", "start")
	_ = os.Setenv("NGINX_UI_SERVER_CA_DIR", "/test/ca")
	_ = os.Setenv("NGINX_UI_SERVER_DEMO", "true")
	_ = os.Setenv("NGINX_UI_SERVER_PAGE_SIZE", "20")
	_ = os.Setenv("NGINX_UI_SERVER_GITHUB_PROXY", "http://proxy.example.com")
	_ = os.Setenv("NGINX_UI_SERVER_CERT_RENEWAL_INTERVAL", "14")
	_ = os.Setenv("NGINX_UI_SERVER_RECURSIVE_NAMESERVERS", "8.8.8.8,1.1.1.1")
	_ = os.Setenv("NGINX_UI_SERVER_SKIP_INSTALLATION", "true")
	_ = os.Setenv("NGINX_UI_SERVER_NAME", "test")

	migrateEnv()

	assert.Equal(t, "127.0.0.1", os.Getenv("NGINX_UI_SERVER_HOST"))
	assert.Equal(t, "8080", os.Getenv("NGINX_UI_SERVER_PORT"))
	// assert.Equal(t, "testing", os.Getenv("NGINX_UI_SERVER_RUN_MODE"))
	assert.Equal(t, "newSecret123", os.Getenv("NGINX_UI_APP_JWT_SECRET"))
	assert.Equal(t, "newSercet123", os.Getenv("NGINX_UI_NODE_SECRET"))
	assert.Equal(t, "9181", os.Getenv("NGINX_UI_CERT_HTTP_CHALLENGE_PORT"))
	assert.Equal(t, "test", os.Getenv("NGINX_UI_CERT_EMAIL"))
	assert.Equal(t, "testDB", os.Getenv("NGINX_UI_DATABASE_NAME"))
	assert.Equal(t, "start", os.Getenv("NGINX_UI_TERMINAL_START_CMD"))
	assert.Equal(t, "/test/ca", os.Getenv("NGINX_UI_CERT_CA_DIR"))
	assert.Equal(t, "true", os.Getenv("NGINX_UI_NODE_DEMO"))
	assert.Equal(t, "20", os.Getenv("NGINX_UI_APP_PAGE_SIZE"))
	assert.Equal(t, "http://proxy.example.com", os.Getenv("NGINX_UI_HTTP_GITHUB_PROXY"))
	assert.Equal(t, "14", os.Getenv("NGINX_UI_CERT_RENEWAL_INTERVAL"))
	assert.Equal(t, "8.8.8.8,1.1.1.1", os.Getenv("NGINX_UI_CERT_RECURSIVE_NAMESERVERS"))
	assert.Equal(t, "true", os.Getenv("NGINX_UI_NODE_SKIP_INSTALLATION"))
	assert.Equal(t, "test", os.Getenv("NGINX_UI_NODE_NAME"))
}

func TestMigration(t *testing.T) {
	const confName = "app.testing.ini"
	confText := `[server]
HttpPort             = 9000
RunMode              = debug
JwtSecret            = newSecret
Email                = test
HTTPChallengePort    = 9181
StartCmd             = bash
Database             = database
CADir                = /test
GithubProxy          = https://mirror.ghproxy.com/
Secret               = newSecret
Demo                 = false
PageSize             = 20
HttpHost             = 0.0.0.0
CertRenewalInterval  = 7
RecursiveNameservers = 8.8.8.8,1.1.1.1
SkipInstallation     = false
Name                 = Local
InsecureSkipVerify   = true

[nginx]
AccessLogPath   =
ErrorLogPath    =
ConfigDir       =
PIDPath         =
ReloadCmd       =
RestartCmd      =
TestConfigCmd   =
LogDirWhiteList = /var/log/nginx

[openai]
Model   = gpt-4o
BaseUrl =
Proxy   =
Token   =

[casdoor]
Endpoint        = http://127.0.0.1:8001
ClientId        = 1234567890qwertyuiop
ClientSecret    = 1234567890qwertyuiop1234567890qwertyuiop
CertificatePath = ./casdoor.pub
Organization    = built-in
Application     = nginx-ui-dev
RedirectUri     =

[logrotate]
Enabled  = true
CMD      = logrotate /etc/logrotate.d/nginx
Interval = 1440

[cluster]
Node = http://10.0.0.1:9000?name=test&node_secret=asdfghjklqwertyuiopzxcvbnm&enabled=true

[auth]
IPWhiteList         = 127.0.0.1
BanThresholdMinutes = 10
MaxAttempts         = 10

[crypto]
Secret = 12345678901234567890

[webauthn]
RPDisplayName = PrimeWaf
RPID          = localhost
RPOrigins     = http://localhost:3002,http://127.0.0.1:3002`
	err := os.WriteFile(confName, []byte(confText), 0644)
	if err != nil {
		t.Fatalf("Failed to write config to file: %v", err)
	}

	migrate(confName)
}
