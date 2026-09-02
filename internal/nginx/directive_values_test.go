package nginx

import (
	"strings"
	"testing"
)

func TestRewriteDirectiveValuesPreservesUnrelatedSource(t *testing.T) {
	content := `# ssl_certificate /commented/old.cer;
server {
    ssl_certificate "/etc/nginx/ssl/example.com_P256/fullchain.cer"; # keep comment
	ssl_certificate_key '/etc/nginx/ssl/example.com_P256/private.key';
    proxy_ssl_certificate /etc/nginx/ssl/example.com_P256/fullchain.cer;
}
`

	rewritten, count, err := RewriteDirectiveValues(content, []DirectiveValueReplacement{
		{Directive: "ssl_certificate", OldValue: "/etc/nginx/ssl/example.com_P256/fullchain.cer", NewValue: "/etc/nginx/ssl/example.com_EC256/fullchain.cer"},
		{Directive: "ssl_certificate_key", OldValue: "/etc/nginx/ssl/example.com_P256/private.key", NewValue: "/etc/nginx/ssl/example.com_EC256/private.key"},
	})
	if err != nil {
		t.Fatalf("RewriteDirectiveValues() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("RewriteDirectiveValues() count = %d, want 2", count)
	}
	if !strings.Contains(rewritten, `ssl_certificate "/etc/nginx/ssl/example.com_EC256/fullchain.cer"; # keep comment`) {
		t.Fatalf("certificate directive was not rewritten in place:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, `ssl_certificate_key '/etc/nginx/ssl/example.com_EC256/private.key';`) {
		t.Fatalf("key directive was not rewritten in place:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, `# ssl_certificate /commented/old.cer;`) ||
		!strings.Contains(rewritten, `proxy_ssl_certificate /etc/nginx/ssl/example.com_P256/fullchain.cer;`) {
		t.Fatalf("unrelated source changed:\n%s", rewritten)
	}
}

func TestDirectiveValuesIgnoresCommentsAndOtherDirectives(t *testing.T) {
	content := `
# ssl_certificate /commented/cert.pem;
server {
    ssl_certificate /first/cert.pem;
    proxy_ssl_certificate /proxy/cert.pem;
    ssl_certificate "/second/cert.pem";
}
`

	values, err := DirectiveValues(content, "ssl_certificate")
	if err != nil {
		t.Fatalf("DirectiveValues() error = %v", err)
	}
	if len(values) != 2 || values[0] != "/first/cert.pem" || values[1] != "/second/cert.pem" {
		t.Fatalf("DirectiveValues() = %#v", values)
	}
}
