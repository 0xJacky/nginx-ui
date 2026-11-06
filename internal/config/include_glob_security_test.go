package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/uozi-tech/cosy"
)

func TestIncludeGlobDotEntriesAreValidated(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	if err := os.MkdirAll(filepath.Join(confDir, "conf.d", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "evil.inc"), "js_import m.js;\n")
	writeIncludeTestFile(t, filepath.Join(confDir, "top.inc"), "perl_modules /tmp/perl;\n")

	// A second temp dir shares the parent of confDir, so ".." can reach it.
	outside := t.TempDir()
	if filepath.Dir(outside) != filepath.Dir(confDir) {
		t.Fatalf("temp dirs do not share a parent: %s, %s", outside, confDir)
	}
	writeIncludeTestFile(t, filepath.Join(outside, "x.inc"), "user root;\n")
	writeIncludeTestFile(t, filepath.Join(outside, "a", "evil.inc"), "perl_set $x 'sub { 1 }';\n")
	if err := os.MkdirAll(filepath.Join(outside, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "a", "b"), filepath.Join(confDir, "conf.d", "link")); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		pattern   string
		wantParam string
	}{
		{"conf.d/su*/../evil.inc", "js_import"},
		// "*" matches both "link" and "sub"; "link" sorts first.
		{"conf.d/*/../evil.inc", "perl_set"},
		{"conf.d/s?b/../evil.inc", "js_import"},
		{"conf.d/.*/top.inc", "perl_modules"},
		{"conf.d/.?/top.inc", "perl_modules"},
		{"conf.d/.*/conf.d/evil.inc", "js_import"},
		{"conf.d/sub/.*/evil.inc", "js_import"},
		{"conf.d/*/../../../" + filepath.Base(outside) + "/x.inc", "user root"},
		// ".." from a symlinked directory leaves through the link target.
		{"conf.d/l*/../evil.inc", "perl_set"},
	}
	for _, tc := range cases {
		t.Run("relative/"+tc.pattern, func(t *testing.T) {
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, "include "+tc.pattern+";\n"), tc.wantParam)
		})
		t.Run("absolute/"+tc.pattern, func(t *testing.T) {
			content := "include " + confDir + "/" + tc.pattern + ";\n"
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, content), tc.wantParam)
		})
	}
}

func TestIncludeGlobMatchesBytesNotRunes(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "é.inc"), "js_import m.js;\n")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "ü", "e.inc"), "js_import m.js;\n")

	// nginx runs glob(3) in the C locale, where "?" and "[...]" match one byte.
	for _, pattern := range []string{
		"conf.d/??.inc",
		"conf.d/[é][é].inc",
		"conf.d/?[^x].inc",
		"conf.d/??/e.inc",
		"conf.d/[ü][ü]/?.inc",
	} {
		t.Run(pattern, func(t *testing.T) {
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, "include "+pattern+";\n"), "js_import")
			content := "include " + confDir + "/" + pattern + ";\n"
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, content), "js_import")
		})
	}
}

func TestIncludeGlobKeepsGlibcDotfileAndMissingBehavior(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "nginx.conf")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "a.conf"), "# ok\n")
	// glob(3) without GLOB_PERIOD never lets a wildcard match a leading
	// dot, so nginx does not load these files either.
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", ".disabled.conf"), "js_import m.js;\n")
	writeIncludeTestFile(t, filepath.Join(confDir, "sites-enabled", ".swp"), "js_import m.js;\n")

	for _, content := range []string{
		"include conf.d/*.conf;\n",
		"include " + confDir + "/conf.d/*.conf;\n",
		"include conf.d/[!.]*.conf;\n",
		"include conf.d/?*.conf;\n",
		"include sites-enabled/*;\n",
		"include missing.d/*/x.conf;\n",
		"include conf.d/missing/*.conf;\n",
	} {
		t.Run(content, func(t *testing.T) {
			if err := ValidateConfigFile(path, content); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	t.Run("explicit dot still matches", func(t *testing.T) {
		requireRestrictedDirectiveError(t, ValidateConfigFile(path, "include conf.d/.*.conf;\n"), "js_import")
	})

	t.Run("malformed pattern in a missing directory", func(t *testing.T) {
		if err := ValidateConfigFile(path, "include missing.d/[]a]*.conf;\n"); err == nil {
			t.Fatal("expected malformed pattern error")
		}
	})
}

// nginx resolves include patterns with glob(3) and opens arguments without
// "*?[" literally. The validator resolves them with Go's Glob, so any pattern
// the two read differently must either be translated or rejected; otherwise
// the files nginx loads through it are never validated.

func requireIncludeGlobUnsupportedError(t *testing.T, err error, wantPattern string) {
	t.Helper()
	cosyErr, ok := errors.AsType[*cosy.Error](err)
	if !ok {
		t.Fatalf("expected cosy error, got %v", err)
	}
	wantErr, _ := errors.AsType[*cosy.Error](ErrConfigIncludeGlobUnsupported)
	if cosyErr.Scope != wantErr.Scope || cosyErr.Code != wantErr.Code {
		t.Fatalf("expected cosy error %s:%d, got %s:%d (%v)", wantErr.Scope, wantErr.Code, cosyErr.Scope, cosyErr.Code, err)
	}
	if len(cosyErr.Params) != 1 || cosyErr.Params[0] != wantPattern {
		t.Fatalf("expected params [%q], got %v", wantPattern, cosyErr.Params)
	}
}

func TestIncludeGlobBracketExpressionsAreRejected(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "mod-perl.inc"), "js_import m.js;\n")

	for _, pattern := range []string{
		"mod-[[:alpha:]]erl.inc",
		"[[:alnum:]]od-perl.inc",
		"mod-[[:lower:]]erl.inc",
		"mod-[[.p.]]erl.inc",
		"mod-[[=p=]]erl.inc",
		"mod-[x[:alpha:]]erl.inc",
		"mod-[![:digit:]]erl.inc",
		"mod-[^[:digit:]]erl.inc",
		"mod-[a-[.z.]]erl.inc",
		"*/mod-[[:alpha:]]erl.inc",
	} {
		t.Run("absolute/"+pattern, func(t *testing.T) {
			full := filepath.Join(confDir, "conf.d", pattern)
			requireIncludeGlobUnsupportedError(t, ValidateConfigFile(path, "include "+full+";\n"), full)
		})
	}

	t.Run("relative", func(t *testing.T) {
		requireIncludeGlobUnsupportedError(t,
			ValidateConfigFile(path, "include conf.d/mod-[[:alpha:]]erl.inc;\n"),
			filepath.Join(confDir, "conf.d", "mod-[[:alpha:]]erl.inc"))
	})

	t.Run("nested include", func(t *testing.T) {
		inner := filepath.Join(confDir, "conf.d", "inner.inc")
		full := filepath.Join(confDir, "conf.d", "mod-[[:alpha:]]erl.inc")
		writeIncludeTestFile(t, inner, "include "+full+";\n")
		requireIncludeGlobUnsupportedError(t, ValidateConfigFile(path, "include "+inner+";\n"), full)
	})
}

func TestIncludeLiteralBackslashIsValidated(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", `mod-\perl.inc`), "js_import m.js;\n")

	// Without any of "*?[" nginx opens the argument as is, and its tokenizer
	// keeps a backslash unless a quote, a backslash, t, r or n follows.
	dir := filepath.Join(confDir, "conf.d")
	for name, content := range map[string]string{
		"unquoted":          "include " + dir + `/mod-\perl.inc;` + "\n",
		"double quoted":     `include "` + dir + `/mod-\perl.inc";` + "\n",
		"single quoted":     `include '` + dir + `/mod-\perl.inc';` + "\n",
		"escaped backslash": "include " + dir + `/mod-\\perl.inc;` + "\n",
		// nginx reads "\\\\" as "\\", which glob(3) matches as one backslash.
		"glob": "include " + dir + `/mod-\\\\perl.in?;` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, content), "js_import")
		})
	}
}

func TestIncludeGlobSupportedPatternsAreValidated(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "mod-perl.inc"), "js_import m.js;\n")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "[lit]!.inc"), "js_import m.js;\n")

	for _, pattern := range []string{
		"mod-perl.inc",
		"mod-*.inc",
		"mod-????.inc",
		"mod-[!x]erl.inc",
		"mod-[^x]erl.inc",
		"mod-[a-z]erl.inc",
		"mod-[p]erl.inc",
		"[!.]od-perl.inc",
		"mod-[.p]erl.inc",
		"mod-[:p]erl.inc",
		`mod-\perl.in?`,
		`\[lit\]!.in?`,
	} {
		t.Run(pattern, func(t *testing.T) {
			content := "include " + filepath.Join(confDir, "conf.d", pattern) + ";\n"
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, content), "js_import")
		})
	}
}

func TestIncludeGlobKeepsStockLayoutsValid(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	for _, name := range []string{
		"mime.types",
		"fastcgi_params",
		"conf.d/default.conf",
		"sites-enabled/default",
		"sites-enabled/example.com",
		"modules-enabled/50-mod-http-geoip.conf",
		"snippets/ssl-params.conf",
		"conf.d/.hidden.conf",
	} {
		writeIncludeTestFile(t, filepath.Join(confDir, name), "# stock file\n")
	}

	for name, content := range map[string]string{
		"stock nginx.conf": "user nginx;\nevents {}\nhttp {\n    include mime.types;\n    include " +
			filepath.Join(confDir, "conf.d") + "/*.conf;\n}\n",
		"debian nginx.conf": "include " + filepath.Join(confDir, "modules-enabled") + "/*.conf;\nevents {}\nhttp {\n" +
			"    include " + filepath.Join(confDir, "mime.types") + ";\n" +
			"    include " + filepath.Join(confDir, "conf.d") + "/*.conf;\n" +
			"    include " + filepath.Join(confDir, "sites-enabled") + "/*;\n}\n",
		"relative includes": "http { include mime.types; include conf.d/*.conf; include sites-enabled/*; include snippets/ssl-params.conf; }\n",
		"site fastcgi":      "server { location ~ \\.php$ { include fastcgi_params; } }\n",
		"glob classes":      "http { include conf.d/[!.]*.conf; include conf.d/[a-z]*.conf; include sites-enabled/[^x]*; include sites-enabled/?xample.com; }\n",
		"missing targets":   "http { include missing.d/*.conf; include missing.conf; include conf.d/[[]missing; }\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateConfigFile(filepath.Join(confDir, "nginx.conf"), content); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestVisitStatementsWithIncludesUsesGlibcGlobSyntax(t *testing.T) {
	root := useDataValidationRoot(t)
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "a.conf"), "ssl_certificate a.cer;\n")
	writeIncludeTestFile(t, filepath.Join(root, "snippets", `b\c.conf`), "ssl_certificate b.cer;\n")

	content := "include snippets/[!.]*.conf;\ninclude " + filepath.Join(root, "snippets", `b\c.conf`) +
		";\ninclude snippets/[[:alpha:]].conf;\ninclude snippets/.?/snippets/a.conf;\ninclude snippets/??.cer;\n"
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "é.cer"), "ssl_certificate e.cer;\n")
	got := collectIncludedStatements("", content)
	want := []string{
		"0:include snippets/[!.]*.conf",
		"1:ssl_certificate a.cer",
		"1:ssl_certificate b.cer",
		"0:include " + filepath.Join(root, "snippets", `b\c.conf`),
		"1:ssl_certificate b.cer",
		"0:include snippets/[[:alpha:]].conf",
		"0:include snippets/.?/snippets/a.conf",
		"1:ssl_certificate a.cer",
		"0:include snippets/??.cer",
		"1:ssl_certificate e.cer",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statements = %#v, want %#v", got, want)
	}
}

func TestGlibcGlobPattern(t *testing.T) {
	for pattern, want := range map[string]string{
		"conf.d/*.conf":   "conf.d/*.conf",
		"[!.]*.conf":      "[^.]*.conf",
		"[^x]":            "[^x]",
		"[a!]":            "[a!]",
		"[a[!]":           "[a[!]",
		`\[!x]`:           `\[!x]`,
		`[\!x]`:           `[\!x]`,
		"[.]*.conf":       "[.]*.conf",
		"[:]":             "[:]",
		"a[!b]c[!d]":      "a[^b]c[^d]",
		"unterminated[!a": "unterminated[^a",
	} {
		got, err := glibcGlobPattern(pattern)
		if err != nil || got != want {
			t.Errorf("glibcGlobPattern(%q) = %q, %v; want %q", pattern, got, err, want)
		}
	}
	for _, pattern := range []string{"[[:alpha:]]", "[a[.b.]]", "[[=e=]]", "[![:space:]]", `[\][:alpha:]]`} {
		if _, err := glibcGlobPattern(pattern); err == nil {
			t.Errorf("glibcGlobPattern(%q) = nil error, want rejection", pattern)
		}
	}
}
