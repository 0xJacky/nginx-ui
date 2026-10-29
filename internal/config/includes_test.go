package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeIncludeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func collectIncludedStatements(path, content string) []string {
	var statements []string
	VisitStatementsWithIncludes(path, []byte(content), func(statement []string, depth int) {
		statements = append(statements, fmt.Sprintf("%d:%s", depth, strings.Join(statement, " ")))
	})
	return statements
}

func TestVisitStatementsWithIncludesSplicesIncludedFilesInPlace(t *testing.T) {
	root := useDataValidationRoot(t)
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "a.conf"), "ssl_certificate a.cer;\n")
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "b.conf"), "include snippets/nested.inc;\n")
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "nested.inc"), "ssl_certificate_key \"b.key\";\n")

	content := `server {
    listen 443 ssl;
    include snippets/*.conf;
    include missing.d/*.conf;
    include missing.conf;
    include $variable.conf;
    server_name example.com;
}`
	got := collectIncludedStatements("", content)
	want := []string{
		"0:server",
		"0:listen 443 ssl",
		"0:include snippets/*.conf",
		"1:ssl_certificate a.cer",
		"1:include snippets/nested.inc",
		"2:ssl_certificate_key b.key",
		"0:include missing.d/*.conf",
		"0:include missing.conf",
		"0:include $variable.conf",
		"0:server_name example.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statements = %#v, want %#v", got, want)
	}
}

func TestVisitStatementsWithIncludesStopsAtCyclesAndDepth(t *testing.T) {
	root := useDataValidationRoot(t)
	sitePath := filepath.Join(root, "sites-available", "example")
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "a.conf"), "include snippets/b.conf;\nssl_certificate a.cer;\n")
	writeIncludeTestFile(t, filepath.Join(root, "snippets", "b.conf"), "include snippets/a.conf;\ninclude sites-available/example;\n")
	siteContent := "include snippets/a.conf;\n"
	writeIncludeTestFile(t, sitePath, siteContent)

	got := collectIncludedStatements(sitePath, siteContent)
	want := []string{
		"0:include snippets/a.conf",
		"1:include snippets/b.conf",
		"2:include snippets/a.conf",
		"2:include sites-available/example",
		"1:ssl_certificate a.cer",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statements = %#v, want %#v", got, want)
	}

	for i := 0; i < maxConfigIncludeDepth+2; i++ {
		writeIncludeTestFile(t, filepath.Join(root, fmt.Sprintf("depth%d.conf", i)),
			fmt.Sprintf("include depth%d.conf;", i+1))
	}
	deepest := 0
	VisitStatementsWithIncludes("", []byte("include depth0.conf;"), func(_ []string, depth int) {
		deepest = max(deepest, depth)
	})
	if deepest != maxConfigIncludeDepth {
		t.Fatalf("deepest include level = %d, want %d", deepest, maxConfigIncludeDepth)
	}
}
