package utils

import (
	"reflect"
	"testing"
)

func TestScanLogDirectives(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		content string
		want    []LogDirective
	}{
		{
			name:   "basic access and error logs",
			prefix: "/etc/nginx",
			content: `server {
    access_log /var/log/nginx/example.access.log;
    error_log /var/log/nginx/example.error.log;
}`,
			want: []LogDirective{
				{Type: "access", Path: "/var/log/nginx/example.access.log"},
				{Type: "error", Path: "/var/log/nginx/example.error.log"},
			},
		},
		{
			name:    "access log with format and options",
			prefix:  "/etc/nginx",
			content: `access_log /var/log/nginx/access.log main buffer=32k flush=5s;`,
			want: []LogDirective{
				{Type: "access", Path: "/var/log/nginx/access.log"},
			},
		},
		{
			name:    "relative path resolved against prefix",
			prefix:  "/usr/local/nginx",
			content: `access_log logs/access.log;`,
			want: []LogDirective{
				{Type: "access", Path: "/usr/local/nginx/logs/access.log"},
			},
		},
		{
			name:   "commented directives skipped",
			prefix: "/etc/nginx",
			content: `server {
    # access_log /var/log/nginx/commented.log;
	#error_log /var/log/nginx/commented.error.log;
    access_log /var/log/nginx/real.log;
}`,
			want: []LogDirective{
				{Type: "access", Path: "/var/log/nginx/real.log"},
			},
		},
		{
			name:   "commented duplicate before real directive",
			prefix: "/etc/nginx",
			content: `# access_log /var/log/nginx/same.log;
access_log /var/log/nginx/same.log;`,
			want: []LogDirective{
				{Type: "access", Path: "/var/log/nginx/same.log"},
			},
		},
		{
			name:   "special targets skipped",
			prefix: "/etc/nginx",
			content: `server {
    access_log off;
    error_log stderr;
    error_log stdout;
    access_log syslog:server=192.168.1.1 main;
    access_log memory:32m;
}`,
			want: []LogDirective{},
		},
		{
			name:    "no directives",
			prefix:  "/etc/nginx",
			content: `server { listen 80; }`,
			want:    []LogDirective{},
		},
		{
			name:   "multiple servers with own logs",
			prefix: "/etc/nginx",
			content: `server {
    access_log /var/log/nginx/a.access.log;
}
server {
    access_log /var/log/nginx/b.access.log combined;
    error_log /var/log/nginx/b.error.log warn;
}`,
			want: []LogDirective{
				{Type: "access", Path: "/var/log/nginx/a.access.log"},
				{Type: "access", Path: "/var/log/nginx/b.access.log"},
				{Type: "error", Path: "/var/log/nginx/b.error.log"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanLogDirectives(tt.prefix, []byte(tt.content))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ScanLogDirectives() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCommentedAt(t *testing.T) {
	content := []byte("    # access_log /tmp/a.log;\naccess_log /tmp/b.log;")

	commentedOffset := 6 // position of "access_log" on the commented line
	if !isCommentedAt(content, commentedOffset) {
		t.Error("expected directive after # to be treated as commented")
	}

	uncommentedOffset := 29 // position of "access_log" on the second line
	if isCommentedAt(content, uncommentedOffset) {
		t.Error("expected directive at line start to be treated as not commented")
	}
}
