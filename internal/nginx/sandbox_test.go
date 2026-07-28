package nginx

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func withSandboxPaths(t *testing.T, files map[string]string, fn func(confDir string, sandboxDir string)) {
	t.Helper()

	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfigPath := settings.NginxSettings.ConfigPath
	originalContainerName := settings.NginxSettings.ContainerName
	originalTestConfigCmd := settings.NginxSettings.TestConfigCmd

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
		settings.NginxSettings.ContainerName = originalContainerName
		settings.NginxSettings.TestConfigCmd = originalTestConfigCmd
	})

	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.ConfigPath = filepath.Join(confDir, "nginx.conf")
	settings.NginxSettings.ContainerName = ""
	settings.NginxSettings.TestConfigCmd = ""
	if files == nil {
		files = make(map[string]string)
	}

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

		expected := "    include " + quoteNginxPath(filepath.Join(sandboxDir, "fastcgi.conf")) + ";"
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
		"fragments/php.conf":                 "include fastcgi.conf;\n# conf-prefix copy\n",
		"sites-available/fragments/php.conf": "include fastcgi.conf;\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		sourcePath := filepath.Join(confDir, "sites-available", "example.conf")
		content := "include fragments/php.conf;\n"

		rewritten, err := builder.rewriteConfigContent(content, sourcePath)
		if err != nil {
			t.Fatalf("rewriteConfigContent() error = %v", err)
		}

		expectedInclude := filepath.Join(sandboxDir, "fragments", "php.conf")
		if !strings.Contains(rewritten, expectedInclude) {
			t.Fatalf("rewriteConfigContent() = %q, want include %q", rewritten, expectedInclude)
		}

		nestedPath := filepath.Join(sandboxDir, "fragments", "php.conf")
		nestedContent, err := os.ReadFile(nestedPath)
		if err != nil {
			t.Fatalf("read mirrored nested dependency: %v", err)
		}

		expectedNestedInclude := filepath.Join(sandboxDir, "fastcgi.conf")
		if !strings.Contains(string(nestedContent), expectedNestedInclude) {
			t.Fatalf("nested dependency = %q, want include %q", string(nestedContent), expectedNestedInclude)
		}
		if !strings.Contains(string(nestedContent), "conf-prefix copy") {
			t.Fatalf("nested dependency = %q, want conf-prefix file", string(nestedContent))
		}
	})
}

func TestResolveIncludePathPrefersConfPrefix(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"snippets/ssl.conf":                 "# conf-prefix\n",
		"sites-available/snippets/ssl.conf": "# sibling\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		resolved, _, err := builder.resolveIncludePath("snippets/ssl.conf", filepath.Join(confDir, "sites-available"))
		if err != nil {
			t.Fatalf("resolveIncludePath() error = %v", err)
		}
		if want := filepath.Join(confDir, "snippets", "ssl.conf"); resolved != want {
			t.Fatalf("resolveIncludePath() = %q, want conf-prefix path %q", resolved, want)
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
		"sites-available/example.conf": "include missing-fastcgi.conf;\n",
		"sites-enabled/example.conf":   "include missing-fastcgi.conf;\n",
	}, func(confDir string, _ string) {
		result := SandboxTestConfigWithPaths(&NamespaceInfo{Name: "demo"}, []string{filepath.Join(confDir, "sites-available", "example.conf")}, nil)

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
	if result.SandboxReason != SandboxReasonRemoteNamespace || result.Message != "" {
		t.Fatalf("result = %+v, want machine-readable remote skip without backend prose", result)
	}
}

func TestResolveIncludePathAllowsEmptyGlob(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"nginx.conf":   "events {}\nhttp { include conf.d/*.conf; }\n",
		"conf.d/.keep": "",
	}, func(_ string, _ string) {
		sandbox, err := createSandbox(&NamespaceInfo{Name: "demo"}, nil, nil)
		if err != nil {
			t.Fatalf("createSandbox() error = %v", err)
		}
		defer sandbox.Cleanup()

		content, err := os.ReadFile(sandbox.ConfigPath)
		if err != nil {
			t.Fatalf("read sandbox config: %v", err)
		}
		if !strings.Contains(string(content), filepath.Join(sandbox.Dir, "conf.d", "*.conf")) {
			t.Fatalf("sandbox config = %q, want rewritten empty glob", string(content))
		}

		builder := newSandboxBuilder(sandbox.Dir)
		absoluteGlob := filepath.Join(builder.confBase, "missing.d", "*.conf")
		resolved, matches, err := builder.resolveIncludePath(absoluteGlob, builder.confBase)
		if err != nil || resolved != absoluteGlob || len(matches) != 0 {
			t.Fatalf("resolveIncludePath(%q) = %q, %v, %v", absoluteGlob, resolved, matches, err)
		}
	})
}

func TestResolveIncludePathRejectsMissingLiteralInclude(t *testing.T) {
	withSandboxPaths(t, nil, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		_, _, err := builder.resolveIncludePath("missing.conf", confDir)
		var sandboxErr *SandboxBuildError
		if !errors.As(err, &sandboxErr) || sandboxErr.Category != ErrorCategoryMissingInclude {
			t.Fatalf("resolveIncludePath() error = %v, want missing include", err)
		}
	})
}

func TestCreateSandboxNeverWritesOutsideSandboxDir(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"nginx.conf":    "events {}\nhttp { include conf.d/*.conf; }\n",
		"mime.types":    "types { text/html html; }\n",
		"http.d/x.conf": "include mime.types;\n",
	}, func(confDir string, _ string) {
		confDPath := filepath.Join(confDir, "conf.d")
		if err := os.Symlink(filepath.Join(confDir, "http.d"), confDPath); err != nil {
			t.Fatalf("symlink conf.d: %v", err)
		}
		realPath := filepath.Join(confDir, "http.d", "x.conf")
		before, err := os.ReadFile(realPath)
		if err != nil {
			t.Fatalf("read source before sandbox: %v", err)
		}

		sandbox, err := createSandbox(&NamespaceInfo{Name: "empty"}, nil, nil)
		if err != nil {
			t.Fatalf("createSandbox() error = %v", err)
		}
		info, err := os.Lstat(filepath.Join(sandbox.Dir, "conf.d"))
		if err != nil {
			t.Fatalf("lstat sandbox conf.d: %v", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("sandbox conf.d must not point at the live configuration tree")
		}
		sandbox.Cleanup()

		after, err := os.ReadFile(realPath)
		if err != nil {
			t.Fatalf("read source after sandbox: %v", err)
		}
		if string(after) != string(before) {
			t.Fatalf("source changed from %q to %q", before, after)
		}
	})
}

func TestReplaceIncludeDirectivesBlockShapes(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"conf.d/foo.conf": "map $host $value { default ok; }\n",
	}, func(confDir string, sandboxDir string) {
		sitesInclude := "    include " + quoteNginxPath(filepath.Join(sandboxDir, "sites-enabled", "a.conf")) + ";"
		streamsInclude := "    include " + quoteNginxPath(filepath.Join(sandboxDir, "streams-enabled", "s.conf")) + ";"
		sitesWildcard := filepath.Join(confDir, "sites-enabled", "*")
		streamsWildcard := filepath.Join(confDir, "streams-enabled", "*")

		tests := []struct {
			name          string
			config        string
			streamInclude bool
		}{
			{name: "Allman http", config: "events {}\nhttp\n{\n include " + sitesWildcard + ";\n}\n"},
			{name: "one line http", config: "events {}\nhttp { include " + sitesWildcard + "; }\n"},
			{name: "include and close", config: "events {}\nhttp {\n include conf.d/foo.conf; }\n"},
			{name: "quoted closing brace", config: "events {}\nhttp { server { return 200 '}'; } include " + sitesWildcard + "; }\n"},
			{name: "multiple quoted braces", config: "events {}\nhttp { server { add_header X-Debug '}}'; } include " + sitesWildcard + "; }\n"},
			{name: "comment brace", config: "events {}\nhttp { server { listen 80; # }\n } include " + sitesWildcard + "; }\n"},
			{
				name:          "stream before http",
				config:        "events {}\nstream { include " + streamsWildcard + "; }\nhttp { include " + sitesWildcard + "; }\n",
				streamInclude: true,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				streamLines := []string(nil)
				if test.streamInclude {
					streamLines = []string{streamsInclude}
				}
				out, err := replaceIncludeDirectives(test.config, filepath.Join(confDir, "nginx.conf"), newSandboxBuilder(sandboxDir), []string{sitesInclude}, streamLines)
				if err != nil {
					t.Fatalf("replaceIncludeDirectives() error = %v", err)
				}
				if strings.Count(out, filepath.Join(sandboxDir, "sites-enabled", "a.conf")) != 1 {
					t.Fatalf("site include count in %q is not one", out)
				}
				assertIncludeInsideTopLevelBlock(t, out, filepath.Join(sandboxDir, "sites-enabled", "a.conf"), "http")
				if strings.Contains(out, sitesWildcard) || strings.Contains(out, streamsWildcard) {
					t.Fatalf("live enabled wildcard survived in %q", out)
				}
				if test.streamInclude {
					assertIncludeInsideTopLevelBlock(t, out, filepath.Join(sandboxDir, "streams-enabled", "s.conf"), "stream")
				}
			})
		}
	})
}

func assertIncludeInsideTopLevelBlock(t *testing.T, content, includePath, expectedBlock string) {
	t.Helper()
	tokens, err := tokenizeNginxConfig(content)
	if err != nil {
		t.Fatalf("tokenizeNginxConfig() error = %v", err)
	}
	stack := make([]string, 0, 4)
	statementStart := 0
	for i, token := range tokens {
		if i == statementStart && token.value == "include" && i+1 < len(tokens) && tokens[i+1].value == includePath {
			if len(stack) != 1 || stack[0] != expectedBlock {
				t.Fatalf("include %q is inside stack %v, want [%s]", includePath, stack, expectedBlock)
			}
			return
		}
		switch token.value {
		case "{":
			name := ""
			if statementStart < i {
				name = strings.ToLower(tokens[statementStart].value)
			}
			stack = append(stack, name)
			statementStart = i + 1
		case "}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			statementStart = i + 1
		case ";":
			statementStart = i + 1
		}
	}
	t.Fatalf("include %q not found in %q", includePath, content)
}

func TestGenerateSandboxConfigFailsWhenIncludesNotInjected(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"nginx.conf":                   "events {}\n",
		"sites-available/example.conf": "server { listen 80; }\n",
		"sites-enabled/example.conf":   "server { listen 80; }\n",
	}, func(confDir string, _ string) {
		result := SandboxTestConfigWithPaths(
			&NamespaceInfo{Name: "demo"},
			[]string{filepath.Join(confDir, "sites-available", "example.conf")},
			nil,
		)
		if result.SandboxStatus != SandboxStatusFailed || result.ErrorCategory != ErrorCategorySandboxBuildError {
			t.Fatalf("result = %+v, want sandbox build failure", result)
		}
		if !strings.Contains(result.Message, "not emitted inside an http block") {
			t.Fatalf("message = %q, want missing emission detail", result.Message)
		}
	})
}

func TestSitesEnabledClassificationUsesResolvedPath(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"conf.d/sites-enabled.conf":           "upstream backend { server 127.0.0.1:8080; }\n",
		"my-sites-enabled-backup/backup.conf": "map $host $backup { default ok; }\n",
	}, func(confDir string, sandboxDir string) {
		config := fmt.Sprintf(`events {}
http {
    include %s;
    include %s; # legacy sites-enabled loader
    include %s;
    include %s;
}
`, filepath.Join(confDir, "conf.d", "sites-enabled.conf"), filepath.Join(confDir, "conf.d", "*.conf"), filepath.Join(confDir, "my-sites-enabled-backup", "*.conf"), filepath.Join(confDir, "sites-enabled", "*"))
		selectedPath := filepath.Join(sandboxDir, "sites-enabled", "selected.conf")
		out, err := replaceIncludeDirectives(
			config,
			filepath.Join(confDir, "nginx.conf"),
			newSandboxBuilder(sandboxDir),
			[]string{"    include " + quoteNginxPath(selectedPath) + ";"},
			nil,
		)
		if err != nil {
			t.Fatalf("replaceIncludeDirectives() error = %v", err)
		}
		if !strings.Contains(out, filepath.Join(sandboxDir, "conf.d", "sites-enabled.conf")) {
			t.Fatalf("unrelated include was removed from %q", out)
		}
		if !strings.Contains(out, filepath.Join(sandboxDir, "my-sites-enabled-backup", "*.conf")) {
			t.Fatalf("backup include was misclassified in %q", out)
		}
		if !strings.Contains(out, "# legacy sites-enabled loader") {
			t.Fatalf("trailing comment was rewritten in %q", out)
		}
		if strings.Count(out, selectedPath) != 1 {
			t.Fatalf("selected include count in %q is not one", out)
		}
	})
}

func TestNestedSitesEnabledIncludeDoesNotLeakForeignNamespaces(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"nginx.conf":                    "events {}\nhttp { include conf.d/*.conf; }\n",
		"conf.d/vhosts.conf":            "include sites-enabled/*;\n",
		"sites-available/selected.conf": "server { listen 8080; }\n",
		"sites-enabled/selected.conf":   "server { listen 8080; }\n",
		"sites-available/foreign.conf":  "server { listen 8081; }\n",
		"sites-enabled/foreign.conf":    "server { listen 8081; }\n",
	}, func(confDir string, _ string) {
		sandbox, err := createSandbox(
			&NamespaceInfo{Name: "selected"},
			[]string{filepath.Join(confDir, "sites-available", "selected.conf")},
			nil,
		)
		if err != nil {
			t.Fatalf("createSandbox() error = %v", err)
		}
		defer sandbox.Cleanup()

		entries, err := os.ReadDir(filepath.Join(sandbox.Dir, "sites-enabled"))
		if err != nil {
			t.Fatalf("read sandbox sites-enabled: %v", err)
		}
		if len(entries) != 1 || entries[0].Name() != "selected.conf" {
			t.Fatalf("sandbox sites-enabled entries = %v, want selected.conf only", entries)
		}
		mainContent, err := os.ReadFile(sandbox.ConfigPath)
		if err != nil {
			t.Fatalf("read sandbox main config: %v", err)
		}
		if strings.Count(string(mainContent), filepath.Join(sandbox.Dir, "sites-enabled", "selected.conf")) != 1 {
			t.Fatalf("selected site was not included exactly once in %q", mainContent)
		}
		dependency, err := os.ReadFile(filepath.Join(sandbox.Dir, "conf.d", "vhosts.conf"))
		if err != nil {
			t.Fatalf("read mirrored dependency: %v", err)
		}
		if strings.Contains(string(dependency), "sites-enabled") {
			t.Fatalf("nested live enabled include survived in %q", dependency)
		}
	})
}

func TestOutOfBaseIncludeDoesNotReenterEnabledTree(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"sites-available/selected.conf": "server { listen 8080; }\n",
		"sites-enabled/selected.conf":   "server { listen 8080; }\n",
	}, func(confDir string, _ string) {
		externalDir := t.TempDir()
		externalPath := filepath.Join(externalDir, "extra.conf")
		externalContent := "include " + filepath.Join(confDir, "sites-enabled", "*") + ";\n"
		if err := os.WriteFile(externalPath, []byte(externalContent), 0644); err != nil {
			t.Fatalf("write external config: %v", err)
		}
		mainContent := "events {}\nhttp { include " + externalPath + "; }\n"
		if err := os.WriteFile(filepath.Join(confDir, "nginx.conf"), []byte(mainContent), 0644); err != nil {
			t.Fatalf("write nginx.conf: %v", err)
		}

		sandbox, err := createSandbox(
			&NamespaceInfo{Name: "selected"},
			[]string{filepath.Join(confDir, "sites-available", "selected.conf")},
			nil,
		)
		if err != nil {
			t.Fatalf("createSandbox() error = %v", err)
		}
		defer sandbox.Cleanup()

		hash := sha256.Sum256([]byte(externalDir))
		mirroredExternal := filepath.Join(sandbox.Dir, "__external", fmt.Sprintf("%x", hash[:8]), "extra.conf")
		content, err := os.ReadFile(mirroredExternal)
		if err != nil {
			t.Fatalf("read mirrored external include: %v", err)
		}
		if strings.Contains(string(content), "sites-enabled") {
			t.Fatalf("external include re-entered enabled tree: %q", content)
		}
	})
}

func TestRewriteIncludeLineHandlesQuotedPaths(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"mime.types":   "types { text/html html; }\n",
		"foo bar.conf": "map $host $value { default ok; }\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		content := "include \"mime.types\"; include 'mime.types'; include \"" + filepath.Join(confDir, "foo bar.conf") + "\";"
		out, err := builder.rewriteConfigContent(content, filepath.Join(confDir, "nginx.conf"))
		if err != nil {
			t.Fatalf("rewriteConfigContent() error = %v", err)
		}
		if strings.Count(out, quoteNginxPath(filepath.Join(sandboxDir, "mime.types"))) != 2 {
			t.Fatalf("quoted mime.types includes were not preserved in %q", out)
		}
		if !strings.Contains(out, quoteNginxPath(filepath.Join(sandboxDir, "foo bar.conf"))) {
			t.Fatalf("quoted spaced include was not rewritten in %q", out)
		}

		_, err = builder.rewriteConfigContent("include foo bar.conf;", filepath.Join(confDir, "nginx.conf"))
		var sandboxErr *SandboxBuildError
		if !errors.As(err, &sandboxErr) || sandboxErr.Category != ErrorCategorySyntaxError {
			t.Fatalf("unquoted spaced include error = %v, want syntax error", err)
		}
	})
}

func TestRewriteIncludeLineMultipleIncludesPerLine(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"a.conf": "map $host $a { default a; }\n",
		"b.conf": "map $host $b { default b; }\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		comment := "# include untouched.conf;"
		out, err := builder.rewriteConfigContent("include a.conf; include b.conf; "+comment, filepath.Join(confDir, "nginx.conf"))
		if err != nil {
			t.Fatalf("rewriteConfigContent() error = %v", err)
		}
		for _, name := range []string{"a.conf", "b.conf"} {
			expected := filepath.Join(sandboxDir, name)
			if strings.Count(out, expected) != 1 {
				t.Fatalf("include %s count in %q is not one", name, out)
			}
			if _, err := os.Stat(expected); err != nil {
				t.Fatalf("mirrored %s missing: %v", name, err)
			}
		}
		if !strings.Contains(out, comment) {
			t.Fatalf("comment changed in %q", out)
		}
	})
}

func TestRewriteIncludeLineDollarInPathIsLiteral(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"a.conf": "map $host $a { default a; }\n",
	}, func(confDir string, _ string) {
		sandboxDir := filepath.Join(t.TempDir(), "u$1000")
		if err := os.MkdirAll(sandboxDir, 0755); err != nil {
			t.Fatalf("mkdir sandbox: %v", err)
		}
		out, err := newSandboxBuilder(sandboxDir).rewriteIncludeLine("include a.conf;", confDir)
		if err != nil {
			t.Fatalf("rewriteIncludeLine() error = %v", err)
		}
		if !strings.Contains(out, sandboxDir) {
			t.Fatalf("dollar-bearing sandbox path was changed in %q", out)
		}
	})
}

func TestGeneratedIncludeLinesAreQuoted(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"sites-available/my site.conf": "server { listen 8080; }\n",
		"sites-enabled/my site.conf":   "server { listen 8080; }\n",
	}, func(confDir string, _ string) {
		sandbox, err := createSandbox(
			&NamespaceInfo{Name: "demo"},
			[]string{filepath.Join(confDir, "sites-available", "my site.conf")},
			nil,
		)
		if err != nil {
			t.Fatalf("createSandbox() error = %v", err)
		}
		defer sandbox.Cleanup()
		content, err := os.ReadFile(sandbox.ConfigPath)
		if err != nil {
			t.Fatalf("read sandbox config: %v", err)
		}
		expected := "include " + quoteNginxPath(filepath.Join(sandbox.Dir, "sites-enabled", "my site.conf")) + ";"
		if !strings.Contains(string(content), expected) {
			t.Fatalf("sandbox config = %q, want %q", content, expected)
		}
	})
}

func TestCopyConfigBaseSkipsUnreadableDanglingAndSpecialEntries(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"mime.types":       "types { text/html html; }\n",
		"conf.d/good.conf": "map $host $value { default ok; }\n",
	}, func(confDir string, sandboxDir string) {
		if err := os.Symlink(filepath.Join(confDir, "missing-file"), filepath.Join(confDir, "dangling-file")); err != nil {
			t.Fatalf("create dangling file symlink: %v", err)
		}
		if err := os.Symlink(filepath.Join(confDir, "missing-dir"), filepath.Join(confDir, "dangling-dir")); err != nil {
			t.Fatalf("create dangling directory symlink: %v", err)
		}
		unreadableDir := filepath.Join(confDir, "private")
		if err := os.Mkdir(unreadableDir, 0000); err != nil {
			t.Fatalf("create unreadable directory: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(unreadableDir, 0755) })

		specialEntry, err := createSandboxSpecialEntry(confDir)
		if err != nil {
			t.Fatalf("create special entry: %v", err)
		}

		if err := copyConfigBaseExceptSitesStreams(sandboxDir); err != nil {
			t.Fatalf("copyConfigBaseExceptSitesStreams() error = %v", err)
		}
		for _, path := range []string{"mime.types", filepath.Join("conf.d", "good.conf")} {
			if _, err := os.Stat(filepath.Join(sandboxDir, path)); err != nil {
				t.Fatalf("readable config %s was not copied: %v", path, err)
			}
		}
		skippedEntries := []string{"dangling-file", "dangling-dir"}
		if specialEntry != "" {
			skippedEntries = append(skippedEntries, specialEntry)
		}
		for _, path := range skippedEntries {
			if _, err := os.Lstat(filepath.Join(sandboxDir, path)); !os.IsNotExist(err) {
				t.Fatalf("unavailable entry %s should be skipped, err = %v", path, err)
			}
		}
	})
}

func TestCopyConfigBaseToleratesConcurrentDeletion(t *testing.T) {
	withSandboxPaths(t, nil, func(confDir string, sandboxDir string) {
		stalePath := filepath.Join(confDir, "already-deleted.conf")
		if err := copySandboxConfigEntry(stalePath, filepath.Join(sandboxDir, "already-deleted.conf"), sandboxDir, filepath.Join(confDir, "nginx.conf"), map[string]bool{}, false); err != nil {
			t.Fatalf("copySandboxConfigEntry() error = %v for a concurrently deleted entry", err)
		}
	})
}

func TestCollectAndCopyIncludesMaintenanceSites(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"sites-available/example.conf":                    "server { listen 8080; }\n",
		"sites-enabled/example.conf_nginx_ui_maintenance": "server { listen 8080; return 503; }\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		if err := os.MkdirAll(filepath.Join(sandboxDir, "sites-enabled"), 0755); err != nil {
			t.Fatalf("mkdir sandbox sites-enabled: %v", err)
		}
		siteFiles, _, err := collectAndCopyNamespaceEnabled(
			&NamespaceInfo{Name: "demo"},
			[]string{filepath.Join(confDir, "sites-available", "example.conf")},
			nil,
			builder,
		)
		if err != nil {
			t.Fatalf("collectAndCopyNamespaceEnabled() error = %v", err)
		}
		if len(siteFiles) != 1 || siteFiles[0] != "example.conf" {
			t.Fatalf("siteFiles = %v, want stable example.conf destination", siteFiles)
		}
		content, err := os.ReadFile(filepath.Join(sandboxDir, "sites-enabled", "example.conf"))
		if err != nil {
			t.Fatalf("read maintenance sandbox config: %v", err)
		}
		if !strings.Contains(string(content), "return 503") {
			t.Fatalf("sandbox content = %q, want maintenance config", content)
		}
	})
}

func TestCollectAndCopyPreservesNestedSitePaths(t *testing.T) {
	withSandboxPaths(t, map[string]string{
		"sites-available/team-a/app.conf": "server { listen 8080; }\n",
		"sites-enabled/team-a/app.conf":   "server { listen 8080; }\n",
		"sites-available/team-b/app.conf": "server { listen 8081; }\n",
		"sites-enabled/team-b/app.conf":   "server { listen 8081; }\n",
		"sites-enabled/app.conf":          "server { listen 9999; }\n",
	}, func(confDir string, sandboxDir string) {
		builder := newSandboxBuilder(sandboxDir)
		if err := os.MkdirAll(filepath.Join(sandboxDir, "sites-enabled"), 0755); err != nil {
			t.Fatalf("mkdir sandbox sites-enabled: %v", err)
		}
		siteFiles, _, err := collectAndCopyNamespaceEnabled(
			&NamespaceInfo{Name: "demo"},
			[]string{
				filepath.Join(confDir, "sites-available", "team-a", "app.conf"),
				filepath.Join(confDir, "sites-available", "team-b", "app.conf"),
				filepath.Join(confDir, "sites-available", "team-a", "app.conf"),
			},
			nil,
			builder,
		)
		if err != nil {
			t.Fatalf("collectAndCopyNamespaceEnabled() error = %v", err)
		}
		want := []string{filepath.Join("team-a", "app.conf"), filepath.Join("team-b", "app.conf")}
		if fmt.Sprint(siteFiles) != fmt.Sprint(want) {
			t.Fatalf("siteFiles = %v, want %v", siteFiles, want)
		}
		for _, relativePath := range want {
			if _, err := os.Stat(filepath.Join(sandboxDir, "sites-enabled", relativePath)); err != nil {
				t.Fatalf("nested sandbox site %s missing: %v", relativePath, err)
			}
		}
		if _, err := os.Stat(filepath.Join(sandboxDir, "sites-enabled", "app.conf")); !os.IsNotExist(err) {
			t.Fatalf("unrelated top-level app.conf was copied, err = %v", err)
		}
	})
}

func TestSandboxSkippedForContainerNginx(t *testing.T) {
	originalContainerName := settings.NginxSettings.ContainerName
	t.Cleanup(func() { settings.NginxSettings.ContainerName = originalContainerName })
	settings.NginxSettings.ContainerName = "nginx"

	result := SandboxTestConfigWithPaths(&NamespaceInfo{Name: "demo"}, nil, nil)
	if result.SandboxStatus != SandboxStatusSkipped || result.SandboxReason != SandboxReasonSeparateContainer {
		t.Fatalf("result = %+v, want separate-container skip", result)
	}
	if result.ErrorCategory == ErrorCategoryMissingInclude {
		t.Fatalf("container skip was misdiagnosed as missing include: %+v", result)
	}
}

func TestRemoteNamespaceSkipTakesPrecedenceOverTestConfigCmd(t *testing.T) {
	originalCommand := settings.NginxSettings.TestConfigCmd
	t.Cleanup(func() { settings.NginxSettings.TestConfigCmd = originalCommand })
	marker := filepath.Join(t.TempDir(), "custom-command-ran")
	settings.NginxSettings.TestConfigCmd = fmt.Sprintf("touch %q", marker)

	result := SandboxTestConfigWithPaths(&NamespaceInfo{DeployMode: "remote"}, nil, nil)
	if result.SandboxReason != SandboxReasonRemoteNamespace {
		t.Fatalf("result = %+v, want remote namespace reason", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("custom command ran for remote namespace, err = %v", err)
	}
}

func TestCustomTestCommandFailurePreservesErrorLevel(t *testing.T) {
	originalCommand := settings.NginxSettings.TestConfigCmd
	originalContainerName := settings.NginxSettings.ContainerName
	t.Cleanup(func() {
		settings.NginxSettings.TestConfigCmd = originalCommand
		settings.NginxSettings.ContainerName = originalContainerName
	})
	settings.NginxSettings.ContainerName = ""
	settings.NginxSettings.TestConfigCmd = "printf 'nginx: [emerg] unknown directive bad' >&2; exit 1"

	result := SandboxTestConfigWithPaths(&NamespaceInfo{Name: "demo"}, nil, nil)
	if result.Level <= Warn || result.ErrorCategory != ErrorCategorySyntaxError {
		t.Fatalf("result = %+v, want preserved custom-command syntax failure", result)
	}
	if result.SandboxReason != SandboxReasonCustomTestCommand {
		t.Fatalf("SandboxReason = %q, want custom test command", result.SandboxReason)
	}
	if strings.Contains(result.Message, "Sandbox validation skipped because") {
		t.Fatalf("backend English skip literal leaked into message: %q", result.Message)
	}
}

func TestNginxStringCacheSerializesColdLoad(t *testing.T) {
	var cache nginxStringCache
	var loadCount atomic.Int32
	var waitGroup sync.WaitGroup
	for range 32 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if got := cache.get(func() string {
				loadCount.Add(1)
				return "cached"
			}); got != "cached" {
				t.Errorf("cache.get() = %q, want cached", got)
			}
		}()
	}
	waitGroup.Wait()
	if got := loadCount.Load(); got != 1 {
		t.Fatalf("loader called %d times, want once", got)
	}
}
