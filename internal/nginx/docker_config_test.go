package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialDockerConfig_PreservesForwardedHost(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "resources", "docker", "nginx-ui.conf"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	config := strings.Join(strings.Fields(string(content)), " ")
	tests := []struct {
		name string
		want string
	}{
		{
			name: "container port listener",
			want: "listen 80;",
		},
		{
			name: "backend proxy",
			want: "proxy_pass http://127.0.0.1:9000/;",
		},
		{
			name: "forwarded host fallback",
			want: "map $http_x_forwarded_host $forwarded_host { default $http_x_forwarded_host; '' $http_host; }",
		},
		{
			name: "forwarded host header",
			want: "proxy_set_header X-Forwarded-Host $forwarded_host;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(config, tt.want) {
				t.Fatalf("official docker config = %q, want %q", config, tt.want)
			}
		})
	}
}
