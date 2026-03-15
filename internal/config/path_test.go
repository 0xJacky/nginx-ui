package config

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
)

func TestResolveConfPath(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = "/etc/nginx"

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	tests := []struct {
		name    string
		parts   []string
		want    string
		wantErr bool
	}{
		{
			name:  "resolve rooted relative path",
			parts: []string{"/conf.d", "site.conf"},
			want:  "/etc/nginx/conf.d/site.conf",
		},
		{
			name:    "reject traversal path",
			parts:   []string{"../../../../tmp"},
			wantErr: true,
		},
		{
			name:    "reject double encoded traversal after decode",
			parts:   []string{helper.UnescapeURL("..%252F..%252F..%252F..%252Ftest")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveConfPath(tt.parts...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveConfPath(%q) expected error", tt.parts)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveConfPath(%q) unexpected error: %v", tt.parts, err)
			}

			if got != tt.want {
				t.Fatalf("ResolveConfPath(%q) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestResolveAbsoluteOrRelativeConfPath(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = "/etc/nginx"

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "allow absolute path under config root",
			path: "/etc/nginx/nginx.conf",
			want: "/etc/nginx/nginx.conf",
		},
		{
			name:    "reject absolute path outside config root",
			path:    "/etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveAbsoluteOrRelativeConfPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveAbsoluteOrRelativeConfPath(%q) expected error", tt.path)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveAbsoluteOrRelativeConfPath(%q) unexpected error: %v", tt.path, err)
			}

			if got != tt.want {
				t.Fatalf("ResolveAbsoluteOrRelativeConfPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveConfPathInDir(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = "/etc/nginx"

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	tests := []struct {
		name    string
		dir     string
		parts   []string
		want    string
		wantErr bool
	}{
		{
			name:  "allow path inside fixed subdir",
			dir:   "sites-available",
			parts: []string{"example.conf"},
			want:  "/etc/nginx/sites-available/example.conf",
		},
		{
			name:    "reject sibling traversal from subdir",
			dir:     "sites-available",
			parts:   []string{"../nginx.conf"},
			wantErr: true,
		},
		{
			name:    "reject double encoded sibling traversal from subdir",
			dir:     "sites-available",
			parts:   []string{helper.UnescapeURL("..%252Fnginx.conf")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveConfPathInDir(tt.dir, tt.parts...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveConfPathInDir(%q, %q) expected error", tt.dir, tt.parts)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveConfPathInDir(%q, %q) unexpected error: %v", tt.dir, tt.parts, err)
			}

			if got != tt.want {
				t.Fatalf("ResolveConfPathInDir(%q, %q) = %q, want %q", tt.dir, tt.parts, got, tt.want)
			}
		})
	}
}

func TestValidateDeletePath(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = "/etc/nginx"

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	if err := ValidateDeletePath("/etc/nginx"); err != ErrCannotDeleteNginxConfDir {
		t.Fatalf("ValidateDeletePath(%q) = %v, want %v", "/etc/nginx", err, ErrCannotDeleteNginxConfDir)
	}

	if err := ValidateDeletePath("/etc/nginx/conf.d/site.conf"); err != nil {
		t.Fatalf("ValidateDeletePath(%q) unexpected error: %v", "/etc/nginx/conf.d/site.conf", err)
	}
}
