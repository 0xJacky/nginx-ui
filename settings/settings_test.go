package settings

import (
	"github.com/stretchr/testify/assert"
	cSettings "github.com/uozi-tech/cosy/settings"
	"os"
	"testing"
)

func TestSetup(t *testing.T) {
	_ = os.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")

	// Server
	_ = os.Setenv("NGINX_UI_SERVER_HOST", "127.0.0.1")
	_ = os.Setenv("NGINX_UI_SERVER_PORT", "8080")
	_ = os.Setenv("NGINX_UI_SERVER_RUN_MODE", "testing")

	// App
	_ = os.Setenv("NGINX_UI_APP_PAGE_SIZE", "20")
	_ = os.Setenv("NGINX_UI_APP_JWT_SECRET", "newSecret123")

	// Database
	_ = os.Setenv("NGINX_UI_DB_NAME", "testDB")

	// Auth
	_ = os.Setenv("NGINX_UI_AUTH_IP_WHITE_LIST", "127.0.0.1,192.168.1.1")
	_ = os.Setenv("NGINX_UI_AUTH_BAN_THRESHOLD_MINUTES", "20")
	_ = os.Setenv("NGINX_UI_AUTH_MAX_ATTEMPTS", "20")

	// Casdoor
	_ = os.Setenv("NGINX_UI_CASDOOR_ENDPOINT", "https://casdoor.example.com")
	_ = os.Setenv("NGINX_UI_CASDOOR_CLIENT_ID", "clientId")
	_ = os.Setenv("NGINX_UI_CASDOOR_CLIENT_SECRET", "clientSecret")
	_ = os.Setenv("NGINX_UI_CASDOOR_CERTIFICATE_PATH", "cert.pem")
	_ = os.Setenv("NGINX_UI_CASDOOR_ORGANIZATION", "org1")
	_ = os.Setenv("NGINX_UI_CASDOOR_APPLICATION", "app1")
	_ = os.Setenv("NGINX_UI_CASDOOR_REDIRECT_URI", "https://redirect.example.com")

	// Cert
	_ = os.Setenv("NGINX_UI_CERT_EMAIL", "test")
	_ = os.Setenv("NGINX_UI_CERT_CA_DIR", "/test/ca")
	_ = os.Setenv("NGINX_UI_CERT_RENEWAL_INTERVAL", "14")
	_ = os.Setenv("NGINX_UI_CERT_RECURSIVE_NAMESERVERS", "8.8.8.8,1.1.1.1")
	_ = os.Setenv("NGINX_UI_CERT_HTTP_CHALLENGE_PORT", "1080")

	// Cluster
	_ = os.Setenv("NGINX_UI_CLUSTER_NODE",
		"http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true,"+
			"http://10.0.0.2:9000?name=node2&node_secret=my-node-secret&enabled=false")

	// Crypto
	_ = os.Setenv("NGINX_UI_CRYPTO_SECRET", "mySecret")

	// Http
	_ = os.Setenv("NGINX_UI_HTTP_GITHUB_PROXY", "http://proxy.example.com")
	_ = os.Setenv("NGINX_UI_HTTP_INSECURE_SKIP_VERIFY", "true")

	// Logrotate
	_ = os.Setenv("NGINX_UI_LOGROTATE_ENABLED", "true")
	_ = os.Setenv("NGINX_UI_LOGROTATE_CMD", "logrotate /custom/logrotate.conf")
	_ = os.Setenv("NGINX_UI_LOGROTATE_INTERVAL", "60")

	// Nginx
	_ = os.Setenv("NGINX_UI_NGINX_ACCESS_LOG_PATH", "/tmp/nginx/access.log")
	_ = os.Setenv("NGINX_UI_NGINX_ERROR_LOG_PATH", "/tmp/nginx/error.log")
	_ = os.Setenv("NGINX_UI_NGINX_CONFIG_DIR", "/etc/nginx/conf")
	_ = os.Setenv("NGINX_UI_NGINX_PID_PATH", "/var/run/nginx.pid")
	_ = os.Setenv("NGINX_UI_NGINX_TEST_CONFIG_CMD", "nginx -t")
	_ = os.Setenv("NGINX_UI_NGINX_RELOAD_CMD", "nginx -s reload")
	_ = os.Setenv("NGINX_UI_NGINX_RESTART_CMD", "nginx -s restart")
	_ = os.Setenv("NGINX_UI_NGINX_LOG_DIR_WHITE_LIST", "/var/log/nginx")

	// Node
	_ = os.Setenv("NGINX_UI_NODE_NAME", "test")
	_ = os.Setenv("NGINX_UI_NODE_SECRET", "nodeSecret")
	_ = os.Setenv("NGINX_UI_NODE_SKIP_INSTALLATION", "true")
	_ = os.Setenv("NGINX_UI_NODE_DEMO", "true")

	// OpenAI
	_ = os.Setenv("NGINX_UI_OPENAI_MODEL", "gpt4o")
	_ = os.Setenv("NGINX_UI_OPENAI_BASE_URL", "https://api.openai.com")
	_ = os.Setenv("NGINX_UI_OPENAI_PROXY", "https://proxy.openai.com")
	_ = os.Setenv("NGINX_UI_OPENAI_TOKEN", "token123")

	// Terminal
	_ = os.Setenv("NGINX_UI_TERMINAL_START_CMD", "bash")

	// WebAuthn
	_ = os.Setenv("NGINX_UI_WEBAUTHN_RP_DISPLAY_NAME", "WebAuthn")
	_ = os.Setenv("NGINX_UI_WEBAUTHN_RPID", "localhost")
	_ = os.Setenv("NGINX_UI_WEBAUTHN_RP_ORIGINS", "http://localhost:3002")

	Init("../app.example.ini")

	// Server
	assert.Equal(t, "127.0.0.1", cSettings.ServerSettings.Host)
	assert.Equal(t, uint(8080), cSettings.ServerSettings.Port)
	assert.Equal(t, "testing", cSettings.ServerSettings.RunMode)

	// App
	assert.Equal(t, 20, cSettings.AppSettings.PageSize)
	assert.Equal(t, "newSecret123", cSettings.AppSettings.JwtSecret)

	// Database
	assert.Equal(t, "testDB", DatabaseSettings.Name)

	// Auth
	assert.Equal(t, []string{"127.0.0.1", "192.168.1.1"}, AuthSettings.IPWhiteList)
	assert.Equal(t, 20, AuthSettings.BanThresholdMinutes)
	assert.Equal(t, 20, AuthSettings.MaxAttempts)

	// Casdoor
	assert.Equal(t, "https://casdoor.example.com", CasdoorSettings.Endpoint)
	assert.Equal(t, "clientId", CasdoorSettings.ClientId)
	assert.Equal(t, "clientSecret", CasdoorSettings.ClientSecret)
	assert.Equal(t, "cert.pem", CasdoorSettings.CertificatePath)
	assert.Equal(t, "org1", CasdoorSettings.Organization)
	assert.Equal(t, "app1", CasdoorSettings.Application)
	assert.Equal(t, "https://redirect.example.com", CasdoorSettings.RedirectUri)

	// Cert
	assert.Equal(t, "test", CertSettings.Email)
	assert.Equal(t, "1080", CertSettings.HTTPChallengePort)
	assert.Equal(t, "/test/ca", CertSettings.CADir)
	assert.Equal(t, 14, CertSettings.RenewalInterval)
	assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, CertSettings.RecursiveNameservers)

	// Cluster
	assert.Equal(t,
		[]string{
			"http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true",
			"http://10.0.0.2:9000?name=node2&node_secret=my-node-secret&enabled=false"},
		ClusterSettings.Node)

	// Crypto
	assert.Equal(t, "mySecret", CryptoSettings.Secret)

	// Http
	assert.Equal(t, "http://proxy.example.com", HTTPSettings.GithubProxy)
	assert.Equal(t, true, HTTPSettings.InsecureSkipVerify)

	// Logrotate
	assert.Equal(t, true, LogrotateSettings.Enabled)
	assert.Equal(t, "logrotate /custom/logrotate.conf", LogrotateSettings.CMD)
	assert.Equal(t, 60, LogrotateSettings.Interval)

	// Nginx
	assert.Equal(t, "/tmp/nginx/access.log", NginxSettings.AccessLogPath)
	assert.Equal(t, "/tmp/nginx/error.log", NginxSettings.ErrorLogPath)
	assert.Equal(t, "/etc/nginx/conf", NginxSettings.ConfigDir)
	assert.Equal(t, "/var/run/nginx.pid", NginxSettings.PIDPath)
	assert.Equal(t, "nginx -t", NginxSettings.TestConfigCmd)
	assert.Equal(t, "nginx -s reload", NginxSettings.ReloadCmd)
	assert.Equal(t, "nginx -s stop", NginxSettings.RestartCmd)
	assert.Equal(t, []string{"/var/log/nginx"}, NginxSettings.LogDirWhiteList)

	// Node
	assert.Equal(t, "test", NodeSettings.Name)
	assert.Equal(t, "nodeSecret", NodeSettings.Secret)
	assert.Equal(t, true, NodeSettings.SkipInstallation)
	assert.Equal(t, true, NodeSettings.Demo)

	// OpenAI
	assert.Equal(t, "gpt4o", OpenAISettings.Model)
	assert.Equal(t, "https://api.openai.com", OpenAISettings.BaseUrl)
	assert.Equal(t, "https://proxy.openai.com", OpenAISettings.Proxy)
	assert.Equal(t, "token123", OpenAISettings.Token)

	// Terminal
	assert.Equal(t, "bash", TerminalSettings.StartCmd)

	// WebAuthn
	assert.Equal(t, "WebAuthn", WebAuthnSettings.RPDisplayName)
	assert.Equal(t, "localhost", WebAuthnSettings.RPID)
	assert.Equal(t, []string{"http://localhost:3002"}, WebAuthnSettings.RPOrigins)

	os.Clearenv()
}
