package audit

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
)

// The audit entry is this feature's compensating control, so a save that
// lifted the denylist has to be recorded even when the directives cannot be
// listed again. Only a save that held none is silent.
func TestRestrictedDirectiveAuditMessage(t *testing.T) {
	const path = "/etc/nginx/conf.d/app.conf"

	tests := []struct {
		name     string
		found    []config.RestrictedDirective
		complete bool
		want     string
		wantOK   bool
	}{
		{
			name:     "nothing restricted was saved",
			complete: true,
			wantOK:   false,
		},
		{
			name:     "directives in the saved file",
			found:    []config.RestrictedDirective{{Directive: "js_import", File: path}, {Directive: "js_content", File: path}},
			complete: true,
			want:     "js_import, js_content",
			wantOK:   true,
		},
		{
			name:     "a directive in an included file names that file",
			found:    []config.RestrictedDirective{{Directive: "js_import", File: "/etc/nginx/conf.d/njs.conf"}},
			complete: true,
			want:     "js_import (/etc/nginx/conf.d/njs.conf)",
			wantOK:   true,
		},
		{
			name:     "an aborted listing is still recorded",
			complete: false,
			want:     incompleteRestrictedDirectiveList,
			wantOK:   true,
		},
		{
			name:     "a partial listing is recorded as partial",
			found:    []config.RestrictedDirective{{Directive: "js_import", File: path}},
			complete: false,
			want:     "js_import, " + incompleteRestrictedDirectiveList,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, ok := restrictedDirectiveAuditMessage(path, tt.found, tt.complete)
			if ok != tt.wantOK {
				t.Fatalf("expected recorded=%t, got %t", tt.wantOK, ok)
			}
			if ok && message != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, message)
			}
		})
	}
}
