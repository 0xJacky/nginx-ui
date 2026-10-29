package config

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func TestIsPrivateKeyMaterial(t *testing.T) {
	for _, tt := range []struct {
		name    string
		path    string
		content string
		want    bool
	}{
		{"key extension without PEM", "/etc/nginx/ssl/example.KEY", "binary", true},
		{"PKCS#8 key", "/etc/nginx/ssl/key.pem", "-----BEGIN PRIVATE KEY-----\nAA==\n-----END PRIVATE KEY-----\n", true},
		{"RSA key", "/etc/nginx/ssl/key.pem", "-----BEGIN RSA PRIVATE KEY-----\nAA==\n", true},
		{"EC key after a certificate", "/etc/nginx/ssl/bundle.pem", "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n-----BEGIN EC PRIVATE KEY-----\n", true},
		{"encrypted key", "/etc/nginx/ssl/key.pem", "-----BEGIN ENCRYPTED PRIVATE KEY-----\n", true},
		{"OpenSSH key", "/etc/nginx/id_ed25519", "-----BEGIN OPENSSH PRIVATE KEY-----\n", true},
		{"certificate", "/etc/nginx/ssl/example.crt", "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n", false},
		{"public key", "/etc/nginx/ssl/example.pub", "-----BEGIN PUBLIC KEY-----\nAA==\n-----END PUBLIC KEY-----\n", false},
		{"nginx configuration", "/etc/nginx/nginx.conf", "ssl_certificate_key /etc/nginx/ssl/example.key;\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsPrivateKeyMaterial(tt.path, []byte(tt.content)))
		})
	}
}

func TestRedactPrivateKeyBackupKeepsOtherHistory(t *testing.T) {
	key := &model.ConfigBackup{Name: "example.key", FilePath: "/etc/nginx/ssl/example.key", Content: "-----BEGIN PRIVATE KEY-----\n"}
	redacted := RedactPrivateKeyBackup(key)
	require.Equal(t, settings.RedactedSensitiveValue, redacted.Content)
	require.Equal(t, key.FilePath, redacted.FilePath)
	require.Equal(t, "-----BEGIN PRIVATE KEY-----\n", key.Content)

	plain := &model.ConfigBackup{Name: "nginx.conf", FilePath: "/etc/nginx/nginx.conf", Content: "events {}\n"}
	require.Equal(t, "events {}\n", RedactPrivateKeyBackup(plain).Content)
}
