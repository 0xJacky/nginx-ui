package site

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/tufanbarisyildirim/gonginx/parser"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func TestCreateMaintenanceConfig_PreservesForwardedHost(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
	})

	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 80;
    server_name example.com;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf)

	if !strings.Contains(content, "proxy_set_header X-Forwarded-Host $http_host;") {
		t.Fatalf("maintenance config = %q, want forwarded host header preservation", content)
	}

	if !strings.Contains(content, "proxy_set_header X-Forwarded-Proto $scheme;") {
		t.Fatalf("maintenance config = %q, want forwarded proto header preservation", content)
	}

	if !strings.Contains(content, "proxy_pass http://127.0.0.1:9000;") {
		t.Fatalf("maintenance config = %q, want proxy to nginx-ui backend", content)
	}
}
