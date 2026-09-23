package performance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

// stockNginxConf reproduces the nginx.conf that nginx ships in conf/nginx.conf
// and that distributions install as the main configuration file. Every
// commented directive below is present verbatim in that file, including the
// `#keepalive_timeout  0;` that sits right above the real one.
const stockNginxConf = `
#user  nobody;
worker_processes  1;

#error_log  logs/error.log;
#pid        logs/nginx.pid;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    #log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
    #                  '$status $body_bytes_sent "$http_referer" '
    #                  '"$http_user_agent" "$http_x_forwarded_for"';

    #access_log  logs/access.log  main;

    sendfile        on;
    #tcp_nopush     on;

    #keepalive_timeout  0;
    keepalive_timeout  65;

    #gzip  on;

    server {
        listen       80;
        server_name  localhost;
    }
}
`

// debianGzipBlock reproduces the gzip section Debian and Ubuntu ship in
// /etc/nginx/nginx.conf: gzip is on, everything that tunes it is commented out.
const debianGzipBlock = `
worker_processes auto;

events {
	worker_connections 768;
}

http {
	client_max_body_size 64m;

	gzip on;

	# gzip_vary on;
	# gzip_proxied any;
	# gzip_comp_level 6;
	# gzip_min_length 256;
	# gzip_buffers 16 8k;
	# gzip_http_version 1.1;
}
`

func writeConf(t *testing.T, content string) {
	t.Helper()

	confPath := filepath.Join(t.TempDir(), "nginx.conf")
	if err := os.WriteFile(confPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write nginx.conf: %v", err)
	}

	previous := settings.NginxSettings.ConfigPath
	settings.NginxSettings.ConfigPath = confPath
	t.Cleanup(func() {
		settings.NginxSettings.ConfigPath = previous
	})
}

func TestGetNginxWorkerConfigInfo_StockConfIgnoresCommentedDirectives(t *testing.T) {
	writeConf(t, stockNginxConf)

	info, err := GetNginxWorkerConfigInfo()
	if err != nil {
		t.Fatalf("GetNginxWorkerConfigInfo() error = %v", err)
	}

	// `#keepalive_timeout  0;` comes first in the file, so a regex that does not
	// know about comments reports 0 instead of the 65 that is in effect.
	if info.KeepaliveTimeout != "65" {
		t.Errorf("KeepaliveTimeout = %q, want %q", info.KeepaliveTimeout, "65")
	}

	// gzip is only mentioned as `#gzip  on;`, so it is off.
	if info.Gzip != "off" {
		t.Errorf("Gzip = %q, want %q", info.Gzip, "off")
	}

	if info.WorkerProcesses != 1 {
		t.Errorf("WorkerProcesses = %d, want %d", info.WorkerProcesses, 1)
	}

	if info.WorkerConnections != 1024 {
		t.Errorf("WorkerConnections = %d, want %d", info.WorkerConnections, 1024)
	}
}

func TestGetNginxWorkerConfigInfo_DebianGzipBlock(t *testing.T) {
	writeConf(t, debianGzipBlock)

	info, err := GetNginxWorkerConfigInfo()
	if err != nil {
		t.Fatalf("GetNginxWorkerConfigInfo() error = %v", err)
	}

	if info.Gzip != "on" {
		t.Errorf("Gzip = %q, want %q", info.Gzip, "on")
	}

	// gzip_comp_level and gzip_min_length are commented out, so nginx uses its
	// own defaults and the dialog must not offer to write the commented values.
	if info.GzipCompLevel != 1 {
		t.Errorf("GzipCompLevel = %d, want the default %d", info.GzipCompLevel, 1)
	}

	if info.GzipMinLength != 1 {
		t.Errorf("GzipMinLength = %d, want the default %d", info.GzipMinLength, 1)
	}

	// An uncommented directive is still read.
	if info.ClientMaxBodySize != "64m" {
		t.Errorf("ClientMaxBodySize = %q, want %q", info.ClientMaxBodySize, "64m")
	}
}

// uppercaseSizeUnits uses uppercase size suffixes, which nginx accepts but the
// parsing regexes previously missed. When a directive was not matched, the
// default was returned instead and silently written back on save, shrinking
// limits the admin had intentionally raised.
const uppercaseSizeUnits = `
worker_processes 2;

events {
	worker_connections 512;
}

http {
	keepalive_timeout 30;
	gzip on;
	client_max_body_size 50M;
	client_header_buffer_size 4K;
	client_body_buffer_size 16K;
}
`

func TestGetNginxWorkerConfigInfo_UppercaseSizeUnits(t *testing.T) {
	writeConf(t, uppercaseSizeUnits)

	info, err := GetNginxWorkerConfigInfo()
	if err != nil {
		t.Fatalf("GetNginxWorkerConfigInfo() error = %v", err)
	}

	if info.ClientMaxBodySize != "50M" {
		t.Errorf("ClientMaxBodySize = %q, want %q", info.ClientMaxBodySize, "50M")
	}

	if info.ClientHeaderBufferSize != "4K" {
		t.Errorf("ClientHeaderBufferSize = %q, want %q", info.ClientHeaderBufferSize, "4K")
	}

	if info.ClientBodyBufferSize != "16K" {
		t.Errorf("ClientBodyBufferSize = %q, want %q", info.ClientBodyBufferSize, "16K")
	}
}

func TestStripComments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "whole line comment",
			content: "    #gzip  on;\n",
			want:    "    \n",
		},
		{
			name:    "trailing comment",
			content: "gzip on; # enabled\n",
			want:    "gzip on; \n",
		},
		{
			name:    "hash inside double quotes is kept",
			content: "add_header X-Tag \"a#b\";\n",
			want:    "add_header X-Tag \"a#b\";\n",
		},
		{
			name:    "hash inside single quotes is kept",
			content: "log_format m '$a#$b';\n",
			want:    "log_format m '$a#$b';\n",
		},
		{
			name:    "quote inside a comment does not open a string",
			content: "# don't do this\ngzip on; # nor this\n",
			want:    "\ngzip on; \n",
		},
		{
			name:    "an unbalanced quote does not leak into the next line",
			content: "log_format m '$a;\n#gzip on;\n",
			want:    "log_format m '$a;\n\n",
		},
		{
			name:    "content without comments is unchanged",
			content: "worker_processes 4;\nevents {\n}\n",
			want:    "worker_processes 4;\nevents {\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripComments(tt.content); got != tt.want {
				t.Errorf("stripComments(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}
