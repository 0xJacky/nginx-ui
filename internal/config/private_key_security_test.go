package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func TestIsSecretFileContent(t *testing.T) {
	for _, tt := range []struct {
		name    string
		path    string
		content string
		want    bool
	}{
		{"key extension without PEM", "/etc/nginx/ssl/example.KEY", "binary", true},
		{"htpasswd extension", "/etc/nginx/auth/.HTPASSWD", "admin:$apr1$abc$def\n", true},
		{"PKCS#8 key", "/etc/nginx/ssl/key.pem", "-----BEGIN PRIVATE KEY-----\nAA==\n-----END PRIVATE KEY-----\n", true},
		{"RSA key", "/etc/nginx/ssl/key.pem", "-----BEGIN RSA PRIVATE KEY-----\nAA==\n", true},
		{"EC key after a certificate", "/etc/nginx/ssl/bundle.pem", "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n-----BEGIN EC PRIVATE KEY-----\n", true},
		{"encrypted key", "/etc/nginx/ssl/key.pem", "-----BEGIN ENCRYPTED PRIVATE KEY-----\n", true},
		{"OpenSSH key", "/etc/nginx/id_ed25519", "-----BEGIN OPENSSH PRIVATE KEY-----\n", true},
		{"PGP private key", "/etc/nginx/signing.asc", "-----BEGIN PGP PRIVATE KEY BLOCK-----\n", true},
		{"certificate", "/etc/nginx/ssl/example.crt", "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n", false},
		{"public key", "/etc/nginx/ssl/example.pub", "-----BEGIN PUBLIC KEY-----\nAA==\n-----END PUBLIC KEY-----\n", false},
		{"PGP public key", "/etc/nginx/signing.asc", "-----BEGIN PGP PUBLIC KEY BLOCK-----\n", false},
		{"nginx configuration", "/etc/nginx/nginx.conf", "ssl_certificate_key /etc/nginx/ssl/example.key;\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsSecretFileContent(tt.path, []byte(tt.content)))
		})
	}
}

func TestSecretFileReferencesMarkFilesNamedBySecretDirectives(t *testing.T) {
	root := t.TempDir()
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = root
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })

	write := func(name, content string) string {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	derKey := write("ssl/site.der", "\x30\x82binary")
	passphrase := write("ssl/site.pass", "secret-passphrase\n")
	ticketKey := write("ssl/ticket.bin", "ticket")
	users := write("auth/users", "admin:$apr1$abc$def\n")
	upstreamKey := write("ssl/upstream.der", "\x30\x82binary")
	commented := write("ssl/old.pass", "old\n")
	certificate := write("ssl/site.crt", "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n")
	include := write("conf.d/extra.conf", "listen 80;\n")
	linkedKey := filepath.Join(root, "ssl/current.der")
	require.NoError(t, os.Symlink(derKey, linkedKey))
	write("conf.d/site.conf", `server {
    listen 443 ssl;
    ssl_certificate ssl/site.crt;
    ssl_certificate_key ssl/site.der; ssl_password_file "`+passphrase+`";
    ssl_session_ticket_key ssl/ticket.bin;
    auth_basic_user_file auth/users;
    # ssl_password_file ssl/old.pass;
    include conf.d/extra.conf;
    location / {
        proxy_ssl_certificate_key `+upstreamKey+`;
        ssl_certificate_key $ssl_server_name.key;
        ssl_certificate_key data:$cert_key;
    }
}
`)

	references := SecretFileReferences()
	for _, secret := range []string{derKey, passphrase, ticketKey, users, upstreamKey, linkedKey} {
		require.True(t, IsSecretFile(secret, []byte("not pem"), references), secret)
	}
	for _, public := range []string{commented, certificate, include} {
		require.False(t, IsSecretFile(public, []byte("not pem"), references), public)
	}
}

func TestSecretBackupRedactorKeepsOtherHistory(t *testing.T) {
	references := map[string]struct{}{"/etc/nginx/ssl/site.pass": {}}
	redact := SecretBackupRedactor(references)

	key := &model.ConfigBackup{Name: "example.key", FilePath: "/etc/nginx/ssl/example.key", Content: "-----BEGIN PRIVATE KEY-----\n"}
	redacted := redact(key)
	require.Equal(t, settings.RedactedSensitiveValue, redacted.Content)
	require.Equal(t, key.FilePath, redacted.FilePath)
	require.Equal(t, "-----BEGIN PRIVATE KEY-----\n", key.Content)

	passphrase := &model.ConfigBackup{Name: "site.pass", FilePath: "/etc/nginx/ssl/site.pass", Content: "secret\n"}
	require.Equal(t, settings.RedactedSensitiveValue, redact(passphrase).Content)

	plain := &model.ConfigBackup{Name: "nginx.conf", FilePath: "/etc/nginx/nginx.conf", Content: "events {}\n"}
	require.Equal(t, "events {}\n", redact(plain).Content)
}
