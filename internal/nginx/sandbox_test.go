package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func withSandboxPaths(t *testing.T, files map[string]string, fn func(confDir string, sandboxDir string)) {
	t.Helper()

	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfigPath := settings.NginxSettings.ConfigPath

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
	})

	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.ConfigPath = filepath.Join(confDir, "nginx.conf")

	if _, ok := files["nginx.conf"]; !ok {
		files["nginx.conf"] = "events {}\nhttp {\n    include sites-enabled/*;\n}\n"
	}

	for relPath, content := range files {
		path := filepath.Join(confDir, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	fn(confDir, t.TempDir())
}

func TestSandboxBuilderRewriteIncludeLineFallsBackToConfBase(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"fastcgi.conf": "fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)

		line, err := builder.rewriteIncludeLine("    include fastcgi.conf;", filepath.Join(confDir, "sites-available"))
		if err != nil {
			t.Fatalf("rewriteIncludeLine() error = %v", err)
		}

		expected := "    include " + filepath.Join(sandboxDir, "fastcgi.conf") + ";"
		if line != expected {
			t.Fatalf("rewriteIncludeLine() = %q, want %q", line, expected)
		}

		if _, err := os.Stat(filepath.Join(sandboxDir, "fastcgi.conf")); err != nil {
			t.Fatalf("expected mirrored fastcgi.conf: %v", err)
		}
	})
}

func TestSandboxBuilderMirrorsNestedIncludeDependencies(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"fastcgi.conf":                       "fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;\n",
		"sites-available/fragments/php.conf": "include fastcgi.conf;\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		sourcePath := filepath.Join(confDir, "sites-available", "example.conf")
		content := "include fragments/php.conf;\n"

		rewritten, err := builder.rewriteConfigContent(content, sourcePath)
		if err != nil {
			t.Fatalf("rewriteConfigContent() error = %v", err)
		}

		expectedInclude := filepath.Join(sandboxDir, "sites-available", "fragments", "php.conf")
		if !strings.Contains(rewritten, expectedInclude) {
			t.Fatalf("rewriteConfigContent() = %q, want include %q", rewritten, expectedInclude)
		}

		nestedPath := filepath.Join(sandboxDir, "sites-available", "fragments", "php.conf")
		nestedContent, err := os.ReadFile(nestedPath)
		if err != nil {
			t.Fatalf("read mirrored nested dependency: %v", err)
		}

		expectedNestedInclude := filepath.Join(sandboxDir, "fastcgi.conf")
		if !strings.Contains(string(nestedContent), expectedNestedInclude) {
			t.Fatalf("nested dependency = %q, want include %q", string(nestedContent), expectedNestedInclude)
		}
	})
}

func TestReplaceIncludeDirectivesInjectsSandboxIncludes(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"mime.types": "types { text/html html; }\n",
	}, func(confDir string, sandboxDir string) {
		mainConf := `
user  nginx;
events {}

http {
    include       mime.types;
    include       /etc/nginx/sites-enabled/*;
}

stream {
    include /etc/nginx/streams-enabled/*;
}
`
		builder := newSandboxBuilder(sandboxDir)
		out, err := replaceIncludeDirectives(
			mainConf,
			filepath.Join(confDir, "nginx.conf"),
			builder,
			[]string{"    include " + filepath.Join(sandboxDir, "sites-enabled", "a.conf") + ";"},
			[]string{"    include " + filepath.Join(sandboxDir, "streams-enabled", "s1.conf") + ";"},
		)
		if err != nil {
			t.Fatalf("replaceIncludeDirectives() error = %v", err)
		}

		if strings.Contains(out, "/etc/nginx/sites-enabled/*") {
			t.Fatal("sites-enabled wildcard should be replaced by sandbox files")
		}
		if strings.Contains(out, "/etc/nginx/streams-enabled/*") {
			t.Fatal("streams-enabled wildcard should be replaced by sandbox files")
		}
		if !strings.Contains(out, filepath.Join(sandboxDir, "sites-enabled", "a.conf")) {
			t.Fatal("sandbox site include missing")
		}
		if !strings.Contains(out, filepath.Join(sandboxDir, "streams-enabled", "s1.conf")) {
			t.Fatal("sandbox stream include missing")
		}
		if !strings.Contains(out, filepath.Join(sandboxDir, "mime.types")) {
			t.Fatal("main nginx include should be rewritten into sandbox")
		}
	})
}

func TestSandboxTestConfigWithPathsReturnsSandboxFailureWithoutFallback(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"sites-enabled/example.conf": "include missing-fastcgi.conf;\n",
	}, func(_ string, _ string) {
		result := SandboxTestConfigWithPaths(&NamespaceInfo{Name: "demo"}, []string{"/tmp/example.conf"}, nil)

		if result.SandboxStatus != SandboxStatusFailed {
			t.Fatalf("SandboxStatus = %q, want %q", result.SandboxStatus, SandboxStatusFailed)
		}
		if result.ErrorCategory != ErrorCategoryMissingInclude {
			t.Fatalf("ErrorCategory = %q, want %q", result.ErrorCategory, ErrorCategoryMissingInclude)
		}
		if !strings.Contains(result.Message, "Sandbox test setup failed") {
			t.Fatalf("Message = %q, want sandbox setup failure", result.Message)
		}
	})
}

func TestSandboxTestConfigWithPathsSkipsRemoteNamespaces(t *testing.T) {
	result := SandboxTestConfigWithPaths(&NamespaceInfo{DeployMode: "remote"}, nil, nil)

	if result.SandboxStatus != SandboxStatusSkipped {
		t.Fatalf("SandboxStatus = %q, want %q", result.SandboxStatus, SandboxStatusSkipped)
	}
	if result.TestScope != TestScopeNamespaceSandbox {
		t.Fatalf("TestScope = %q, want %q", result.TestScope, TestScopeNamespaceSandbox)
	}
}
