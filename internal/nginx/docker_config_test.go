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

	config := string(content)

	if !strings.Contains(config, "listen       80;") {
		t.Fatalf("official docker config = %q, want container port 80 listener", config)
	}

	if !strings.Contains(config, "proxy_pass http://127.0.0.1:9000/;") {
		t.Fatalf("official docker config = %q, want backend proxy to port 9000", config)
	}

	if !strings.Contains(config, "proxy_set_header   X-Forwarded-Host     $http_host;") {
		t.Fatalf("official docker config = %q, want forwarded host header preservation", config)
	}
}
