package cert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseHTTP01Config(t *testing.T, content string) *nginx.NgxConfig {
	t.Helper()
	parsed, err := nginx.ParseNgxConfigByContent(content)
	require.NoError(t, err)
	return parsed
}

func TestValidateHTTP01ChallengeConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "valid HTTP listener and proxy",
			content: `server {
	listen 80;
	server_name example.com;
	location ~ /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`,
		},
		{
			// ACME validators follow the HTTPS redirect, so it only fails
			// because no HTTPS server routes the challenge.
			name: "server level redirect without HTTPS challenge route",
			content: `server {
	listen 80;
	server_name example.com;
	return 301 https://$host$request_uri;
}`,
			wantErr: "no HTTPS server for this name",
		},
		{
			name: "server level redirect overrides challenge proxy",
			content: `server {
	listen 80;
	server_name example.com;
	return 301 https://$host$request_uri;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`,
			wantErr: "no HTTPS server for this name",
		},
		{
			name: "server level redirect to HTTPS server routing the challenge",
			content: `server {
	listen 80;
	server_name example.com;
	return 301 https://$host$request_uri;
}
server {
	listen 443 ssl;
	server_name example.com;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`,
		},
		{
			name: "location level redirect to HTTPS server routing the challenge",
			content: `server {
	listen 80;
	server_name example.com;
	location / {
	return 301 https://$host$request_uri;
	}
}
server {
	listen 443 ssl;
	server_name example.com;
	location ~ /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`,
		},
		{
			name: "redirect to HTTPS server without challenge route",
			content: `server {
	listen 80;
	server_name example.com;
	location / {
	return 301 https://$host$request_uri;
	}
}
server {
	listen 443 ssl;
	server_name example.com;
	location / {
	proxy_pass http://127.0.0.1:3000;
	}
}`,
			wantErr: "redirects to HTTPS",
		},
		{
			name: "server level non-redirect return",
			content: `server {
	listen 80;
	server_name example.com;
	return 444;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`,
			wantErr: "server-level return",
		},
		{
			name: "challenge only on HTTPS",
			content: `server {
	listen 443 ssl;
	server_name example.com;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`,
			wantErr: "no server listening on port 80",
		},
		{
			name: "wrong challenge port",
			content: `server {
	listen 80;
	server_name example.com;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9181;
	}
}`,
			wantErr: "does not proxy_pass to http://127.0.0.1:9180",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTP01ChallengeConfig(parseHTTP01Config(t, tt.content), "9180", []string{"example.com"})
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "HTTP-01 route is unavailable")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateHTTP01ChallengeConfigRequiresEachIdentifier(t *testing.T) {
	config := parseHTTP01Config(t, `server {
	listen 80;
	server_name example.com;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`)

	assert.NoError(t, validateHTTP01ChallengeConfig(config, "9180", []string{"example.com"}))
	err := validateHTTP01ChallengeConfig(config, "9180", []string{"example.com", "www.example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "www.example.com")
}

func TestValidateHTTP01ChallengeConfigReadsEnabledSite(t *testing.T) {
	configDir := t.TempDir()
	previousConfigDir := settings.NginxSettings.ConfigDir
	previousChallengePort := settings.CertSettings.HTTPChallengePort
	settings.NginxSettings.ConfigDir = configDir
	settings.CertSettings.HTTPChallengePort = "9180"
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = previousConfigDir
		settings.CertSettings.HTTPChallengePort = previousChallengePort
	})

	availableDir := filepath.Join(configDir, "sites-available")
	enabledDir := filepath.Join(configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(availableDir, 0755))
	require.NoError(t, os.MkdirAll(enabledDir, 0755))
	name := "example.com"
	availablePath := filepath.Join(availableDir, name)
	require.NoError(t, os.WriteFile(availablePath, []byte(`server {
	listen 80;
	server_name example.com;
	location /.well-known/acme-challenge {
	proxy_pass http://127.0.0.1:9180;
	}
}`), 0644))

	err := ValidateHTTP01ChallengeConfig(name, []string{name})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")

	require.NoError(t, os.Symlink(availablePath, filepath.Join(enabledDir, name)))
	assert.NoError(t, ValidateHTTP01ChallengeConfig(name, []string{name}))
}
