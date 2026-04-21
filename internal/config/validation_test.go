package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

func TestValidateConfigFilename(t *testing.T) {
	confDir := t.TempDir()
	for _, dir := range []string{
		"conf.d",
		"snippets",
		"sites-available",
		"sites-enabled",
		"streams-available",
		"streams-enabled",
	} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "allow root nginx conf",
			path: filepath.Join(confDir, "nginx.conf"),
		},
		{
			name: "allow standard root text file",
			path: filepath.Join(confDir, "mime.types"),
		},
		{
			name: "allow conf file anywhere",
			path: filepath.Join(confDir, "conf.d", "app.conf"),
		},
		{
			name: "allow site hostname",
			path: filepath.Join(confDir, "sites-available", "example.com"),
		},
		{
			name: "allow stream bare name",
			path: filepath.Join(confDir, "streams-enabled", "tcp_proxy"),
		},
		{
			name:    "reject shared library",
			path:    filepath.Join(confDir, "evil.so"),
			wantErr: true,
		},
		{
			name:    "reject non-conf bare name outside managed dirs",
			path:    filepath.Join(confDir, "conf.d", "evil"),
			wantErr: true,
		},
		{
			name:    "reject dangerous managed extension",
			path:    filepath.Join(confDir, "sites-available", "evil.pl"),
			wantErr: true,
		},
		{
			name:    "reject dangerous snippet extension",
			path:    filepath.Join(confDir, "snippets", "evil.pl"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigFilename(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateConfigFilename(%q) expected error", tt.path)
				}
				var cosyErr *cosy.Error
				if !errors.As(err, &cosyErr) {
					t.Fatalf("ValidateConfigFilename(%q) expected cosy error, got %v", tt.path, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateConfigFilename(%q) unexpected error: %v", tt.path, err)
			}
		})
	}
}

func TestValidateConfigContentBytes(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "allow nginx text",
			content: []byte("server {\n\tlisten 80;\n}\n"),
		},
		{
			name:    "reject invalid utf8",
			content: []byte{0xff, 0xfe, 0xfd},
			wantErr: true,
		},
		{
			name:    "reject null byte",
			content: []byte("server {\x00}\n"),
			wantErr: true,
		},
		{
			name:    "reject control byte",
			content: []byte("server {\x01}\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigContentBytes(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateConfigContentBytes(%q) expected error", tt.content)
				}
				var cosyErr *cosy.Error
				if !errors.As(err, &cosyErr) {
					t.Fatalf("ValidateConfigContentBytes(%q) expected cosy error, got %v", tt.content, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateConfigContentBytes(%q) unexpected error: %v", tt.content, err)
			}
		})
	}
}
