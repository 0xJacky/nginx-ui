package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/uozi-tech/cosy"
)

// debianNginxConf is the Debian nginx.conf layout reported in #2087, with
// /etc/nginx replaced by the test config directory.
func debianNginxConf(confDir string) string {
	return strings.ReplaceAll(`user www-data;
worker_processes auto;
pid /run/nginx.pid;
error_log /var/log/nginx/error.log;
include /etc/nginx/modules-enabled/*.conf;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}

rtmp {
    server {
        listen 1935;
        application hls {
            live on;
            hls on;
            hls_path /var/www/html/hls;
        }
    }
}

stream {
    include /etc/nginx/streams-enabled/*;
}
`, "/etc/nginx", confDir)
}

// enableDebianModule mirrors how Debian's libnginx-mod-* packages enable a
// module: a file in modules-available, linked from modules-enabled.
func enableDebianModule(t *testing.T, confDir, name, module string) {
	t.Helper()
	available := filepath.Join(confDir, "modules-available", "mod-"+name+".conf")
	writeIncludeTestFile(t, available, "load_module modules/"+module+";\n")
	if err := os.MkdirAll(filepath.Join(confDir, "modules-enabled"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(available, filepath.Join(confDir, "modules-enabled", "50-mod-"+name+".conf")); err != nil {
		t.Fatal(err)
	}
}

func requireIncludedDirectiveError(t *testing.T, err error, wantParams []string) {
	t.Helper()
	cosyErr, ok := errors.AsType[*cosy.Error](err)
	if !ok {
		t.Fatalf("expected cosy error, got %v", err)
	}
	if !isCosyError(cosyErr, ErrIncludedDirectiveNotAllowed) {
		t.Fatalf("expected included directive error, got %s:%d %v", cosyErr.Scope, cosyErr.Code, cosyErr.Params)
	}
	if !reflect.DeepEqual(cosyErr.Params, wantParams) {
		t.Fatalf("params = %q, want %q", cosyErr.Params, wantParams)
	}
}

func TestDistroEnabledModulesKeepNginxConfEditable(t *testing.T) {
	confDir := useDataValidationRoot(t)
	enableDebianModule(t, confDir, "rtmp", "ngx_rtmp_module.so")
	enableDebianModule(t, confDir, "http-perl", "ngx_http_perl_module.so")
	enableDebianModule(t, confDir, "http-js", "ngx_http_js_module.so")
	enableDebianModule(t, confDir, "stream-js", "ngx_stream_js_module.so")
	writeIncludeTestFile(t, filepath.Join(confDir, "sites-enabled", "default"), "server { listen 80; root /var/www/html; }\n")

	if err := ValidateConfigFile(filepath.Join(confDir, "nginx.conf"), debianNginxConf(confDir)); err != nil {
		t.Fatalf("nginx.conf with distro-enabled modules rejected: %v", err)
	}
}

func TestEditedFileStillCannotLoadRestrictedModules(t *testing.T) {
	confDir := useDataValidationRoot(t)
	for _, module := range blockedDynamicModules {
		t.Run(module, func(t *testing.T) {
			content := "load_module modules/" + module + ";\n" + debianNginxConf(confDir)
			requireRestrictedDirectiveError(t, ValidateConfigFile(filepath.Join(confDir, "nginx.conf"), content), "load_module")
		})
	}
}

func TestIncludedRestrictedDirectivesNameTheIncludedFile(t *testing.T) {
	confDir := useDataValidationRoot(t)
	nginxConf := filepath.Join(confDir, "nginx.conf")
	enableDebianModule(t, confDir, "http-perl", "ngx_http_perl_module.so")
	enableDebianModule(t, confDir, "http-js", "ngx_http_js_module.so")
	writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "app.conf"), "server { listen 80; include snippets/*.conf; }\n")
	writeIncludeTestFile(t, filepath.Join(confDir, "snippets", "perl.conf"), "perl_require evil.pm;\n")

	// A module loaded by the distribution does not unlock its directives,
	// and the error names the nested file that holds them.
	requireIncludedDirectiveError(t, ValidateConfigFile(nginxConf, debianNginxConf(confDir)),
		[]string{filepath.Join(confDir, "snippets", "perl.conf"), "perl_require"})

	writeIncludeTestFile(t, filepath.Join(confDir, "snippets", "perl.conf"), "add_header X a#; js_import m.js;\n")
	requireIncludedDirectiveError(t, ValidateConfigFile(nginxConf, debianNginxConf(confDir)),
		[]string{filepath.Join(confDir, "snippets", "perl.conf"), "js_import"})

	writeIncludeTestFile(t, filepath.Join(confDir, "snippets", "perl.conf"), "user root;\n")
	requireIncludedDirectiveError(t, ValidateConfigFile(nginxConf, debianNginxConf(confDir)),
		[]string{filepath.Join(confDir, "snippets", "perl.conf"), "user root"})

	writeIncludeTestFile(t, filepath.Join(confDir, "snippets", "perl.conf"), "add_header X-Ok 1;\n")
	if err := ValidateConfigFile(nginxConf, debianNginxConf(confDir)); err != nil {
		t.Fatalf("safe snippet rejected: %v", err)
	}
}

func TestIncludedDirectiveErrorHidesNamesOutsideConfDir(t *testing.T) {
	confDir := useDataValidationRoot(t)
	sitePath := filepath.Join(confDir, "sites-available", "example")

	outside := t.TempDir()
	writeIncludeTestFile(t, filepath.Join(outside, "secret-name.inc"), "js_import m.js;\n")
	pattern := filepath.Join(outside, "*.inc")
	requireIncludedDirectiveError(t, ValidateConfigFile(sitePath, "include "+pattern+";\n"),
		[]string{pattern, "js_import"})

	// A path that leaves the config directory through ".." is reported by
	// the include argument too, even when it cleans to a path inside it.
	if err := os.MkdirAll(filepath.Join(outside, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "dir"), filepath.Join(confDir, "link")); err != nil {
		t.Fatal(err)
	}
	dotted := "link/../secret-name.inc"
	requireIncludedDirectiveError(t, ValidateConfigFile(sitePath, "include "+dotted+";\n"),
		[]string{dotted, "js_import"})
}

func TestEnvPerlVariablesBlockedAtEveryDepth(t *testing.T) {
	confDir := useDataValidationRoot(t)
	nginxConf := filepath.Join(confDir, "nginx.conf")
	enableDebianModule(t, confDir, "http-perl", "ngx_http_perl_module.so")

	// env exporting a Perl runtime variable feeds the embedded interpreter,
	// so it stays blocked whether it is in the edited file or an included
	// one, even though the module load itself is now allowed in includes.
	blocked := []string{
		"PERL5OPT=-Mx",
		"PERL5LIB=/tmp/lib",
		"PERLLIB=/tmp/lib",
		"PERL5DB=x",
		"PERL5OPT",
		"perl5opt=-Mx",
	}
	for _, arg := range blocked {
		t.Run("edited/"+arg, func(t *testing.T) {
			content := "env \"" + arg + "\";\n" + debianNginxConf(confDir)
			requireRestrictedDirectiveError(t, ValidateConfigFile(nginxConf, content), "env "+strings.SplitN(arg, "=", 2)[0])
		})
		t.Run("included/"+arg, func(t *testing.T) {
			writeIncludeTestFile(t, filepath.Join(confDir, "conf.d", "env.conf"), "env \""+arg+"\";\n")
			content := "include conf.d/env.conf;\n" + debianNginxConf(confDir)
			requireIncludedDirectiveError(t, ValidateConfigFile(nginxConf, content),
				[]string{filepath.Join(confDir, "conf.d", "env.conf"), "env " + strings.SplitN(arg, "=", 2)[0]})
			os.Remove(filepath.Join(confDir, "conf.d", "env.conf"))
		})
	}
}

func TestEnvNonPerlVariablesStayAllowed(t *testing.T) {
	confDir := useDataValidationRoot(t)
	nginxConf := filepath.Join(confDir, "nginx.conf")
	for _, arg := range []string{"TZ", "TZ=UTC", "MALLOC_ARENA_MAX=2", "PERSONAL=1"} {
		t.Run(arg, func(t *testing.T) {
			content := "env \"" + arg + "\";\n" + debianNginxConf(confDir)
			if err := ValidateConfigFile(nginxConf, content); err != nil {
				t.Fatalf("env %q rejected: %v", arg, err)
			}
		})
	}
}

func TestIncludedDirectiveErrorHidesSymlinkedOutsideNames(t *testing.T) {
	confDir := useDataValidationRoot(t)
	sitePath := filepath.Join(confDir, "sites-available", "example")

	// A symlinked directory inside the conf dir points outside it, like
	// RHEL's /etc/nginx/modules -> /usr/lib64/nginx/modules. A file reached
	// through it must be named by the include argument, not its real path.
	outside := t.TempDir()
	writeIncludeTestFile(t, filepath.Join(outside, "secret-name.inc"), "js_import m.js;\n")
	if err := os.Symlink(outside, filepath.Join(confDir, "linkdir")); err != nil {
		t.Fatal(err)
	}
	pattern := "linkdir/secret-name.inc"
	requireIncludedDirectiveError(t, ValidateConfigFile(sitePath, "include "+pattern+";\n"),
		[]string{pattern, "js_import"})
}
