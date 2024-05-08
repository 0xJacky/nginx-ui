package settings

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestSetup(t *testing.T) {
	Init("../app.example.ini")

	_ = os.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")

	_ = os.Setenv("NGINX_UI_SERVER_HTTP_PORT", "8080")
	_ = os.Setenv("NGINX_UI_SERVER_RUN_MODE", "test")
	_ = os.Setenv("NGINX_UI_SERVER_JWT_SECRET", "newSecret123")
	_ = os.Setenv("NGINX_UI_SERVER_HTTP_CHALLENGE_PORT", "9181")
	_ = os.Setenv("NGINX_UI_SERVER_START_CMD", "start")
	_ = os.Setenv("NGINX_UI_SERVER_DATABASE", "testDB")
	_ = os.Setenv("NGINX_UI_SERVER_CA_DIR", "/test/ca")
	_ = os.Setenv("NGINX_UI_SERVER_GITHUB_PROXY", "http://proxy.example.com")
	_ = os.Setenv("NGINX_UI_SERVER_NODE_SECRET", "nodeSecret")
	_ = os.Setenv("NGINX_UI_SERVER_DEMO", "true")
	_ = os.Setenv("NGINX_UI_SERVER_PAGE_SIZE", "20")
	_ = os.Setenv("NGINX_UI_SERVER_HTTP_HOST", "127.0.0.1")
	_ = os.Setenv("NGINX_UI_SERVER_CERT_RENEWAL_INTERVAL", "14")
	_ = os.Setenv("NGINX_UI_SERVER_RECURSIVE_NAMESERVERS", "8.8.8.8")
	_ = os.Setenv("NGINX_UI_SERVER_SKIP_INSTALLATION", "true")
	_ = os.Setenv("NGINX_UI_SERVER_NAME", "test")

	_ = os.Setenv("NGINX_UI_NGINX_ACCESS_LOG_PATH", "/tmp/nginx/access.log")
	_ = os.Setenv("NGINX_UI_NGINX_ERROR_LOG_PATH", "/tmp/nginx/error.log")
	_ = os.Setenv("NGINX_UI_NGINX_CONFIG_DIR", "/etc/nginx/conf")
	_ = os.Setenv("NGINX_UI_NGINX_PID_PATH", "/var/run/nginx.pid")
	_ = os.Setenv("NGINX_UI_NGINX_TEST_CONFIG_CMD", "nginx -t")
	_ = os.Setenv("NGINX_UI_NGINX_RELOAD_CMD", "nginx -s reload")
	_ = os.Setenv("NGINX_UI_NGINX_RESTART_CMD", "nginx -s restart")

	_ = os.Setenv("NGINX_UI_OPENAI_MODEL", "davinci")
	_ = os.Setenv("NGINX_UI_OPENAI_BASE_URL", "https://api.openai.com")
	_ = os.Setenv("NGINX_UI_OPENAI_PROXY", "https://proxy.openai.com")
	_ = os.Setenv("NGINX_UI_OPENAI_TOKEN", "token123")

	_ = os.Setenv("NGINX_UI_CASDOOR_ENDPOINT", "https://casdoor.example.com")
	_ = os.Setenv("NGINX_UI_CASDOOR_CLIENT_ID", "clientId")
	_ = os.Setenv("NGINX_UI_CASDOOR_CLIENT_SECRET", "clientSecret")
	_ = os.Setenv("NGINX_UI_CASDOOR_CERTIFICATE", "cert.pem")
	_ = os.Setenv("NGINX_UI_CASDOOR_ORGANIZATION", "org1")
	_ = os.Setenv("NGINX_UI_CASDOOR_APPLICATION", "app1")
	_ = os.Setenv("NGINX_UI_CASDOOR_REDIRECT_URI", "https://redirect.example.com")

	_ = os.Setenv("NGINX_UI_LOGROTATE_ENABLED", "true")
	_ = os.Setenv("NGINX_UI_LOGROTATE_CMD", "logrotate /custom/logrotate.conf")
	_ = os.Setenv("NGINX_UI_LOGROTATE_INTERVAL", "60")

	ConfPath = "app.testing.ini"
	Setup()

	assert.Equal(t, "8080", ServerSettings.HttpPort)
	assert.Equal(t, "test", ServerSettings.RunMode)
	assert.Equal(t, "newSecret123", ServerSettings.JwtSecret)
	assert.Equal(t, "9181", ServerSettings.HTTPChallengePort)
	assert.Equal(t, "start", ServerSettings.StartCmd)
	assert.Equal(t, "testDB", ServerSettings.Database)
	assert.Equal(t, "/test/ca", ServerSettings.CADir)
	assert.Equal(t, "http://proxy.example.com", ServerSettings.GithubProxy)
	assert.Equal(t, "nodeSecret", ServerSettings.NodeSecret)
	assert.Equal(t, true, ServerSettings.Demo)
	assert.Equal(t, 20, ServerSettings.PageSize)
	assert.Equal(t, "127.0.0.1", ServerSettings.HttpHost)
	assert.Equal(t, 14, ServerSettings.CertRenewalInterval)
	assert.Equal(t, []string{"8.8.8.8"}, ServerSettings.RecursiveNameservers)
	assert.Equal(t, true, ServerSettings.SkipInstallation)
	assert.Equal(t, "test", ServerSettings.Name)

	assert.Equal(t, "/tmp/nginx/access.log", NginxSettings.AccessLogPath)
	assert.Equal(t, "/tmp/nginx/error.log", NginxSettings.ErrorLogPath)
	assert.Equal(t, "/etc/nginx/conf", NginxSettings.ConfigDir)
	assert.Equal(t, "/var/run/nginx.pid", NginxSettings.PIDPath)
	assert.Equal(t, "nginx -t", NginxSettings.TestConfigCmd)
	assert.Equal(t, "nginx -s reload", NginxSettings.ReloadCmd)
	assert.Equal(t, "nginx -s stop", NginxSettings.RestartCmd)

	assert.Equal(t, "davinci", OpenAISettings.Model)
	assert.Equal(t, "https://api.openai.com", OpenAISettings.BaseUrl)
	assert.Equal(t, "https://proxy.openai.com", OpenAISettings.Proxy)
	assert.Equal(t, "token123", OpenAISettings.Token)

	assert.Equal(t, "https://casdoor.example.com", CasdoorSettings.Endpoint)
	assert.Equal(t, "clientId", CasdoorSettings.ClientId)
	assert.Equal(t, "clientSecret", CasdoorSettings.ClientSecret)
	assert.Equal(t, "cert.pem", CasdoorSettings.Certificate)
	assert.Equal(t, "org1", CasdoorSettings.Organization)
	assert.Equal(t, "app1", CasdoorSettings.Application)
	assert.Equal(t, "https://redirect.example.com", CasdoorSettings.RedirectUri)

	assert.Equal(t, true, LogrotateSettings.Enabled)
	assert.Equal(t, "logrotate /custom/logrotate.conf", LogrotateSettings.CMD)
	assert.Equal(t, 60, LogrotateSettings.Interval)

	os.Clearenv()
	_ = os.Remove("app.testing.ini")
}
