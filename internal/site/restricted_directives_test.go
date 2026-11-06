package site

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restrictedSiteConfig loads njs into the Nginx worker, which Save refuses
// unless the API handler verified the two-factor step-up first.
const restrictedSiteConfig = "server {\n    listen 80;\n    js_import main from conf.d/main.js;\n    js_content main.handler;\n}\n"

// Without the option nothing changes: the directive is refused and no file is
// left behind. This is what every internal caller keeps doing — a duplicate,
// an upstream conversion, an access list rewrite, a sync from another node.
func TestSaveRejectsRestrictedDirectivesByDefault(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)

	err := Save(context.Background(), "example.com", restrictedSiteConfig, true, 0, nil, "")
	requireSiteRestrictedDirectiveError(t, err, "js_import")

	path := filepath.Join(confDir, "sites-available", "example.com")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to stay absent, stat error: %v", path, statErr)
	}
}

// With the option the same content saves, so an administrator who confirmed
// the step-up can deploy their own njs.
func TestSaveAllowsRestrictedDirectivesWithTheOption(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)
	err := Save(context.Background(), "example.com", restrictedSiteConfig, true, 0, nil, "",
		AllowRestrictedDirectives(true))
	if err != nil {
		t.Fatalf("expected the save to be accepted, got %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(confDir, "sites-available", "example.com"))
	if readErr != nil {
		t.Fatalf("expected the site to be written: %v", readErr)
	}
	if !strings.Contains(string(content), "js_import") {
		t.Fatalf("expected js_import to be saved, got %q", content)
	}
}

// AllowRestrictedDirectives(false) is the explicit form of the default, so a
// handler can pass the gate's decision straight through.
func TestSaveRejectsRestrictedDirectivesWhenTheOptionIsFalse(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)

	err := Save(context.Background(), "example.com", restrictedSiteConfig, true, 0, nil, "",
		AllowRestrictedDirectives(false))
	requireSiteRestrictedDirectiveError(t, err, "js_import")

	path := filepath.Join(confDir, "sites-available", "example.com")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to stay absent, stat error: %v", path, statErr)
	}
}

// The option covers the directives that load code, not the account the
// workers run as.
func TestSaveKeepsRootWorkersBlockedWithTheOption(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)

	err := Save(context.Background(), "example.com", "user root;\nserver {\n    listen 80;\n}\n", true, 0, nil, "",
		AllowRestrictedDirectives(true))
	requireSiteRestrictedDirectiveError(t, err, "user root")

	path := filepath.Join(confDir, "sites-available", "example.com")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to stay absent, stat error: %v", path, statErr)
	}
}
