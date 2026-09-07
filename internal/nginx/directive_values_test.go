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

// TestRewriteDirectiveValuesEscapesBackslash keeps a rewritten value readable
// back as itself, whichever form the original directive used. A backslash is an
// escape inside both quote styles and outside them, so leaving one bare loses a
// character, or lets it swallow the closing quote or the semicolon.
func TestRewriteDirectiveValuesEscapesBackslash(t *testing.T) {
	// The empty value only has somewhere to go once the directive is quoted,
	// so it is not a case for the already-quoted forms.
	quoted := []string{
		`/etc/ssl/a\b/key.pem`,
		`/etc/ssl/a\\b/key.pem`,
		`/etc/ssl/a\tb/key.pem`,
		`/etc/ssl/o'brien/key.pem`,
		`/etc/ssl/say"hi"/key.pem`,
		`/etc/ssl/trailing\`,
	}

	for _, tt := range []struct {
		name     string
		original string
		values   []string
	}{
		{"single quoted", "'/old/key.pem'", quoted},
		{"double quoted", `"/old/key.pem"`, quoted},
		{"unquoted", "/old/key.pem", append(append([]string{}, quoted...), "")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, newValue := range tt.values {
				content := "server {\n    ssl_certificate_key " + tt.original + ";\n}\n"

				rewritten, count, err := RewriteDirectiveValues(content, []DirectiveValueReplacement{
					{Directive: "ssl_certificate_key", OldValue: "/old/key.pem", NewValue: newValue},
				})
				if err != nil {
					t.Errorf("RewriteDirectiveValues(%q) error = %v", newValue, err)
					continue
				}
				if count != 1 {
					t.Errorf("RewriteDirectiveValues(%q) count = %d, want 1", newValue, count)
					continue
				}

				values, err := DirectiveValues(rewritten, "ssl_certificate_key")
				if err != nil {
					t.Errorf("DirectiveValues(%q) error = %v, rewritten:\n%s", newValue, err, rewritten)
					continue
				}
				if len(values) != 1 || values[0] != newValue {
					t.Errorf("round trip gave %#v, want [%q], rewritten:\n%s", values, newValue, rewritten)
				}
			}
		})
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
