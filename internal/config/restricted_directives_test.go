package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/uozi-tech/cosy"
)

// restrictedSaveCases are the statements the two-factor step-up unlocks: the
// njs and Perl directives and the dynamic modules that provide them.
var restrictedSaveCases = []struct {
	name      string
	content   string
	wantParam string
}{
	{"js_import", "server {\n    js_import main from conf.d/main.js;\n}\n", "js_import"},
	{"js_content", "server {\n    location / {\n        js_content main.handler;\n    }\n}\n", "js_content"},
	{"js_fetch_verify", "server {\n    js_fetch_verify off;\n}\n", "js_fetch_verify"},
	{"perl", "server {\n    location / {\n        perl handler::run;\n    }\n}\n", "perl"},
	{"perl_set", "server {\n    perl_set $x 'sub { return 1; }';\n}\n", "perl_set"},
	{"load_module http js", "load_module modules/ngx_http_js_module.so;\nevents {}\n", "load_module"},
	{"load_module stream js", "load_module modules/ngx_stream_js_module.so;\nevents {}\n", "load_module"},
	{"load_module perl", "load_module modules/ngx_http_perl_module.so;\nevents {}\n", "load_module"},
}

// The default entry points must keep rejecting every restricted statement, so
// a caller that cannot step up — a cluster node, a service token, MCP, the
// LLM paths, a duplicate or an upstream conversion — is unaffected.
func TestValidateConfigFileStillRejectsRestrictedDirectives(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")

	for _, tc := range restrictedSaveCases {
		t.Run(tc.name, func(t *testing.T) {
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, tc.content), tc.wantParam)
			requireRestrictedDirectiveError(t, ValidateConfigFileBytes(path, []byte(tc.content)), tc.wantParam)
			requireRestrictedDirectiveError(t, ValidateConfigFileWithRestricted(path, tc.content, false), tc.wantParam)
		})
	}
}

// With the step-up verified, the same statements save.
func TestValidateConfigFileWithRestrictedAcceptsRestrictedDirectives(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")

	for _, tc := range restrictedSaveCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateConfigFileWithRestricted(path, tc.content, true); err != nil {
				t.Fatalf("expected %s to be accepted after the step-up, got %v", tc.wantParam, err)
			}
		})
	}
}

// The step-up unlocks code the administrator deploys into the worker, not the
// account the workers run as. "user root" stays rejected either way.
func TestValidateConfigFileWithRestrictedKeepsRootWorkersBlocked(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "nginx.conf")

	for _, allowRestricted := range []bool{false, true} {
		err := ValidateConfigFileWithRestricted(path, "user root;\nevents {}\n", allowRestricted)
		requireRestrictedDirectiveError(t, err, "user root")

		// "user root" is not the unlockable kind, so it must not carry the
		// error type the API turns into a step-up offer.
		var restricted *RestrictedDirectiveError
		if errors.As(err, &restricted) {
			t.Fatalf("user root must not be reported as an unlockable restricted directive (allowRestricted=%t)", allowRestricted)
		}
	}
}

// Lifting the denylist must not lift anything else: the filename policy and
// the content policy still apply.
func TestValidateConfigFileWithRestrictedKeepsOtherPoliciesIntact(t *testing.T) {
	confDir := setupTokenizerConfDir(t)

	if err := ValidateConfigFileWithRestricted(filepath.Join(confDir, "evil.so"), "events {}\n", true); err == nil {
		t.Fatal("expected the filename policy to still reject evil.so")
	}

	path := filepath.Join(confDir, "conf.d", "app.conf")
	if err := ValidateConfigFileWithRestricted(path, "server {\x00}\n", true); err == nil {
		t.Fatal("expected control characters to still be rejected")
	}
	if err := ValidateConfigFileWithRestricted(path, "server {\n    js_import \xff;\n}\n", true); err == nil {
		t.Fatal("expected invalid UTF-8 to still be rejected")
	}
}

// A rejection must stay the same cosy error for every caller, so the response
// an existing client sees does not change.
func TestRestrictedDirectiveErrorUnwrapsToTheDirectiveError(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")

	err := ValidateConfigFile(path, "server {\n    js_import main from conf.d/main.js;\n}\n")

	var restricted *RestrictedDirectiveError
	if !errors.As(err, &restricted) {
		t.Fatalf("expected a RestrictedDirectiveError, got %v", err)
	}
	if restricted.Found.Directive != "js_import" {
		t.Fatalf("expected js_import, got %q", restricted.Found.Directive)
	}
	if restricted.Found.File != path {
		t.Fatalf("expected the error to name %q, got %q", path, restricted.Found.File)
	}

	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected the error to unwrap to a cosy error, got %v", err)
	}
	var wantErr *cosy.Error
	if !errors.As(ErrConfigDirectiveNotAllowed, &wantErr) {
		t.Fatal("ErrConfigDirectiveNotAllowed is not a cosy error")
	}
	if cosyErr.Scope != wantErr.Scope || cosyErr.Code != wantErr.Code {
		t.Fatalf("expected %s:%d, got %s:%d", wantErr.Scope, wantErr.Code, cosyErr.Scope, cosyErr.Code)
	}
}

// The confirmation dialog lists what a save would enable, so every restricted
// statement is reported once, with the file holding it.
func TestFindRestrictedDirectivesReportsEveryStatementOnce(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")

	found, complete := FindRestrictedDirectives(path, "server {\n    js_import main from conf.d/main.js;\n    js_content main.handler;\n    js_import main from conf.d/main.js;\n}\n")
	if !complete {
		t.Fatal("expected the walk to complete")
	}

	want := []RestrictedDirective{
		{Directive: "js_import", File: path},
		{Directive: "js_content", File: path},
	}
	if len(found) != len(want) {
		t.Fatalf("expected %v, got %v", want, found)
	}
	for index := range want {
		if found[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, found)
		}
	}
}

func TestFindRestrictedDirectivesReportsNothingForAnOrdinaryConfig(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")

	found, complete := FindRestrictedDirectives(path, "server {\n    listen 80;\n    # js_import main.js;\n}\n")
	if !complete {
		t.Fatal("expected the walk to complete")
	}
	if len(found) != 0 {
		t.Fatalf("expected no restricted directives, got %v", found)
	}
}

// A restricted statement in an included file blocks the save of the file that
// includes it, and the step-up accepts the whole tree. The administrator has
// to see which file it came from, since it is not the one they edited.
func TestRestrictedDirectivesInIncludedFiles(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	includedPath := filepath.Join(confDir, "conf.d", "scripting.conf")
	if err := os.WriteFile(includedPath, []byte("js_import main from conf.d/main.js;\n"), 0o644); err != nil {
		t.Fatalf("failed to seed the included file: %v", err)
	}

	path := filepath.Join(confDir, "nginx.conf")
	content := "events {}\nhttp {\n    include conf.d/scripting.conf;\n}\n"

	requireRestrictedDirectiveError(t, ValidateConfigFile(path, content), "js_import")

	if err := ValidateConfigFileWithRestricted(path, content, true); err != nil {
		t.Fatalf("expected the step-up to accept the included directive, got %v", err)
	}

	found, complete := FindRestrictedDirectives(path, content)
	if !complete {
		t.Fatal("expected the walk to complete")
	}
	if len(found) != 1 {
		t.Fatalf("expected one restricted directive, got %v", found)
	}
	if found[0].Directive != "js_import" {
		t.Fatalf("expected js_import, got %q", found[0].Directive)
	}
	if found[0].File != includedPath {
		t.Fatalf("expected the included file %q to be named, got %q", includedPath, found[0].File)
	}
}

// A walk that cannot finish must say so, so the audit does not read it as a
// configuration holding no restricted directive.
func TestFindRestrictedDirectivesReportsAnIncompleteWalk(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	content := "include conf.d/app.conf;\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to seed the config file: %v", err)
	}

	found, complete := FindRestrictedDirectives(path, content)
	if complete {
		t.Fatalf("a cyclic include cannot produce a complete walk, got %v", found)
	}
}

// Hiding a restricted directive behind nginx token boundaries must not get
// past the gate either: the tokenizer cases stay rejected without the
// step-up, so an opt-in is always a conscious choice.
func TestRestrictedDirectiveGateUsesTheSameTokenizer(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")

	for _, tc := range tokenizerAttackCases {
		t.Run(tc.name, func(t *testing.T) {
			requireRestrictedDirectiveError(t, ValidateConfigFileWithRestricted(path, tc.content, false), tc.wantParam)
		})
	}
}
