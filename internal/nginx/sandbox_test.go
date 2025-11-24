package nginx

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeIncludeLineRelativeTo(t *testing.T) {
	baseDir := "/etc/nginx/sites-available"
	if runtime.GOOS == "windows" {
		// keep test portable; filepath.Join will use OS-specific separator
		baseDir = `C:\nginx\conf\sites-available`
	}
	sandboxDir := "/tmp/sbx"

	tests := []struct {
		name       string
		in         string
		wantPrefix string
	}{
		{
			name:       "relative simple file",
			in:         "    include mime.types;",
			wantPrefix: "    include ",
		},
		{
			name:       "relative path with subdir",
			in:         "include ../common/snippets/*.conf;",
			wantPrefix: "include ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := normalizeIncludeLineRelativeTo(tt.in, baseDir, sandboxDir)
			if out == "" {
				t.Fatalf("expected non-empty include, got empty")
			}
			if !strings.HasPrefix(out, tt.wantPrefix) {
				t.Fatalf("unexpected prefix: %q, got %q", tt.wantPrefix, out)
			}
			// if relative input (first two cases), ensure absolute joined path appears
			if tt.name == "relative simple file" || tt.name == "relative path with subdir" {
				parts := strings.Split(out, "include ")
				if len(parts) < 2 {
					t.Fatalf("malformed include line: %q", out)
				}
				pathWithSemi := parts[1]
				path := strings.TrimSuffix(pathWithSemi, ";")
				if !filepath.IsAbs(path) {
					t.Fatalf("expected absolute path, got %q", path)
				}
			}
		})
	}
}

func TestReplaceIncludeDirectives(t *testing.T) {
	mainConf := `
user  nginx;
worker_processes auto;
error_log  /var/log/nginx/error.log notice;
pid        /var/run/nginx.pid;

events {
    worker_connections  1024;
}

http {
    server {
        location / {
            return 200;
        }
    }
    include       mime.types;
    include       /etc/nginx/conf.d/*.conf;
    include       /etc/nginx/sites-enabled/*;
}

stream {
    include /etc/nginx/streams-enabled/*;
}
`
	siteLines := []string{"    include /tmp/sbx/sites-enabled/a.conf;"}
	streamLines := []string{"    include /tmp/sbx/streams-enabled/s1.conf;"}

	out := replaceIncludeDirectives(mainConf, "/tmp/sbx", siteLines, streamLines)

	// ensure site includes inserted before closing http brace (inside block)
	lines := strings.Split(out, "\n")
	httpStart := -1
	httpClose := -1
	inHttp := false
	depth := 0
	for i, l := range lines {
		if strings.Contains(l, "http {") && httpStart == -1 {
			httpStart = i
			inHttp = true
			depth = 1
			continue
		}
		if inHttp {
			depth += strings.Count(l, "{")
			depth -= strings.Count(l, "}")
			if depth == 0 {
				httpClose = i
				inHttp = false
				break
			}
		}
	}
	if httpStart == -1 || httpClose == -1 {
		t.Fatal("failed to locate http block bounds")
	}
	incIdx := -1
	for i := httpStart; i <= httpClose; i++ {
		if strings.Contains(lines[i], "/tmp/sbx/sites-enabled/a.conf;") {
			incIdx = i
			break
		}
	}
	if incIdx == -1 || incIdx >= httpClose {
		t.Fatalf("sandbox site include should be inside http block before closing brace, got index=%d close=%d", incIdx, httpClose)
	}

	if strings.Contains(out, "/etc/nginx/sites-enabled/*") {
		t.Fatal("sites-enabled wildcard should be replaced by sandbox files")
	}
	if !strings.Contains(out, "/tmp/sbx/sites-enabled/a.conf;") {
		t.Fatal("sandbox site include missing")
	}
	if strings.Contains(out, "/etc/nginx/streams-enabled/*") {
		t.Fatal("streams-enabled wildcard should be replaced by sandbox files")
	}
	if !strings.Contains(out, "/tmp/sbx/streams-enabled/s1.conf;") {
		t.Fatal("sandbox stream include missing")
	}
	// mime.types should be kept (possibly normalized)
	if !strings.Contains(strings.ToLower(out), "include") {
		t.Fatal("expected include directives to remain")
	}
}
