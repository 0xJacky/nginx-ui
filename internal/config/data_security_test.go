package config

import (
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func useDataValidationRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	previous := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	t.Cleanup(func() { *settings.NginxSettings = previous })
	return root
}

func TestDataFileRejectsDirectivesBeforeAndAfterExistingInclude(t *testing.T) {
	root := useDataValidationRoot(t)
	payload := "user root;\nload_module /usr/lib/nginx/modules/ngx_http_js_module.so;\n"
	for _, extension := range []string{".pem", ".crt", ".cer", ".key", ".csr", ".crl", ".ca-bundle", ".htpasswd"} {
		t.Run(extension, func(t *testing.T) {
			path := filepath.Join(root, "payload"+extension)
			if err := ValidateConfigFile(path, payload); err == nil {
				t.Fatal("directive payload accepted before include")
			}
			if err := ValidateConfigFileBytes(path, []byte(payload)); err == nil {
				t.Fatal("byte entry point accepted directive payload")
			}
			// Reverse order: an existing include/glob cannot make a later data
			// write bypass the same policy by avoiding a new .conf save.
			if err := ValidateConfigFile(filepath.Join(root, "nginx.conf"), "include "+path+";"); err != nil {
				t.Fatalf("missing include should remain for nginx -t to diagnose: %v", err)
			}
			if err := ValidateConfigFile(path, payload); err == nil {
				t.Fatal("directive payload accepted after include")
			}
		})
	}
}

func TestPEMDataRejectsUnconsumedTextAndPreservesFormats(t *testing.T) {
	root := useDataValidationRoot(t)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1, 2, 3}})
	legacyKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte{1, 2, 3}, Headers: map[string]string{
		"Proc-Type": "4,ENCRYPTED", "DEK-Info": "AES-256-CBC,00112233",
	}})
	for _, content := range [][]byte{certificate, append(append([]byte("# CA bundle\n"), certificate...), certificate...), legacyKey,
		pem.EncodeToMemory(&pem.Block{Type: "DH PARAMETERS", Bytes: []byte{1, 2, 3}})} {
		if err := ValidateConfigFileBytes(filepath.Join(root, "ssl", "custom.pem"), content); err != nil {
			t.Fatalf("valid PEM structure rejected: %v", err)
		}
	}
	for _, content := range []string{
		"user root;\n" + string(certificate), string(certificate) + "\nuser root;\n",
		string(certificate) + "\nnot PEM", "-----BEGIN CERTIFICATE-----\ninvalid\n" + string(certificate),
	} {
		if err := ValidateConfigFile(filepath.Join(root, "bundle.pem"), content); err == nil {
			t.Fatalf("unconsumed PEM text accepted: %q", content)
		}
	}
}

func TestHTPasswdPreservesPasswordPunctuationAndRejectsDirectiveNames(t *testing.T) {
	root := useDataValidationRoot(t)
	path := filepath.Join(root, "auth.htpasswd")
	valid := "# accounts\r\nalice:{PLAIN}a;user root;{literal}\\quotes\"\r\nbob:$apr1$abc$hash\n"
	if err := ValidateConfigFile(path, valid); err != nil {
		t.Fatalf("literal password punctuation rejected: %v", err)
	}
	for _, content := range []string{"user root;#:password\n", "user root;#:\n", "js_import /tmp/a:b;\n", "include /tmp/payload;#:x\n", "alice:valid\nuser root;\n"} {
		if err := ValidateConfigFile(path, content); err == nil {
			t.Fatalf("directive smuggled into username: %q", content)
		}
	}
}

func TestIncludeValidationChecksExistingTargetsAndPreservesSafeSnippets(t *testing.T) {
	root := useDataValidationRoot(t)
	outside := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(outside, "system.inc"), "proxy_set_header Host $host;\n")
	write(filepath.Join(root, "snippets", "safe.conf"), "include \""+filepath.Join(outside, "system.inc")+"\";\ninclude missing.d/*.conf;\n")
	content := "include snippets/*.conf;"
	if err := ValidateConfigFile(filepath.Join(root, "nginx.conf"), content); err != nil {
		t.Fatalf("safe glob, main-prefix relative and external includes rejected: %v", err)
	}
	write(filepath.Join(root, "legacy.key"), "js_import app.js;\n")
	write(filepath.Join(root, "snippets", "safe.conf"), "include legacy.key;\n")
	requireRestrictedDirectiveError(t, ValidateConfigFile(filepath.Join(root, "nginx.conf"), content), "js_import")
	write(filepath.Join(root, "snippets", "safe.conf"), "include snippets/safe.conf;\n")
	if err := ValidateConfigFile(filepath.Join(root, "nginx.conf"), content); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("include cycle must fail closed: %v", err)
	}
}

func TestIncludeValidationBoundsDepthAndChecksSymlinkTargets(t *testing.T) {
	root := useDataValidationRoot(t)
	malicious := filepath.Join(root, "malicious.key")
	if err := os.WriteFile(malicious, []byte("user root;"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.inc")
	if err := os.Symlink(malicious, alias); err == nil {
		requireRestrictedDirectiveError(t, ValidateConfigFile(filepath.Join(root, "nginx.conf"), "include alias.inc;"), "user root")
	}
	for i := 0; i < maxConfigIncludeDepth+2; i++ {
		path := filepath.Join(root, fmt.Sprintf("depth%d.conf", i))
		content := fmt.Sprintf("include depth%d.conf;", i+1)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateConfigFile(filepath.Join(root, "nginx.conf"), "include depth0.conf;"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("include chain exceeded bound without rejection: %v", err)
	}
}
