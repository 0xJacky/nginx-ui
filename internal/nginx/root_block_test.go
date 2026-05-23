package nginx

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNgxDirectiveRawIsInternalOnly(t *testing.T) {
	data, err := json.Marshal(NgxDirective{
		Directive: "ssl_certificate_by_lua_block",
		Params:    "placeholder",
		Raw:       "ssl_certificate_by_lua_block { auto_ssl:ssl_certificate() }",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(data), "raw") || strings.Contains(string(data), "auto_ssl") {
		t.Fatalf("NgxDirective JSON = %s, want Raw omitted from external representation", data)
	}

	var directive NgxDirective
	if err := json.Unmarshal([]byte(`{"directive":"listen","params":"443 ssl","raw":"server_name injected.example.com"}`), &directive); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if directive.Raw != "" {
		t.Fatalf("Raw = %q, want external JSON input ignored", directive.Raw)
	}
}

func TestParseNgxConfigByContent_UnwrapsRootStreamBlock(t *testing.T) {
	content := `stream {
    log_format vless_lb '$remote_addr:$remote_port [$time_local] '
        '$protocol $status '
        'connect=$upstream_connect_time '
        'session=$session_time '
        'sent=$bytes_sent recv=$bytes_received '
        'upstream=$upstream_addr';
    access_log /var/log/nginx/xray_stream_access.log vless_lb;
    error_log /var/log/nginx/xray_stream_error.log warn;

    upstream xray_vless_449 {
        least_conn;
        server 1.1.1.1:449 fail_timeout=15s max_fails=3;
        server 1.1.1.1:449 fail_timeout=15s max_fails=3;
    }

    server {
        listen 449;
        proxy_connect_timeout 5s;
        proxy_timeout 15m;
        proxy_pass xray_vless_449;
    }

    include /etc/nginx/streams-enabled/*;
}`

	ngxConfig, err := ParseNgxConfigByContent(content)
	if err != nil {
		t.Fatalf("ParseNgxConfigByContent() error = %v", err)
	}

	if ngxConfig.RootBlock != Stream {
		t.Fatalf("RootBlock = %q, want %q", ngxConfig.RootBlock, Stream)
	}

	if len(ngxConfig.Upstreams) != 1 {
		t.Fatalf("len(Upstreams) = %d, want 1", len(ngxConfig.Upstreams))
	}

	if len(ngxConfig.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(ngxConfig.Servers))
	}

	if !strings.Contains(ngxConfig.Custom, "log_format vless_lb") {
		t.Fatalf("Custom = %q, want log_format directive", ngxConfig.Custom)
	}

	if !strings.Contains(ngxConfig.Custom, "include /etc/nginx/streams-enabled/*;") {
		t.Fatalf("Custom = %q, want include directive", ngxConfig.Custom)
	}

	builtContent, err := ngxConfig.BuildConfig()
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}

	if !strings.Contains(builtContent, "stream {") {
		t.Fatalf("built content = %q, want stream root block", builtContent)
	}

	if !strings.Contains(builtContent, "upstream xray_vless_449 {") {
		t.Fatalf("built content = %q, want upstream block", builtContent)
	}

	if !strings.Contains(builtContent, "least_conn;") {
		t.Fatalf("built content = %q, want least_conn directive", builtContent)
	}

	if !strings.Contains(builtContent, "server {") {
		t.Fatalf("built content = %q, want server block", builtContent)
	}

	if !strings.Contains(builtContent, "proxy_pass xray_vless_449;") {
		t.Fatalf("built content = %q, want proxy_pass directive", builtContent)
	}
}
