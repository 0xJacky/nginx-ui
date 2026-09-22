package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateLabelsFile verifies the gettext extraction anchor: labels from
// every notifier are deduplicated, sorted and emitted as $gettext literals so
// `gettext:extract` can record them as msgids.
func TestGenerateLabelsFile(t *testing.T) {
	dir := t.TempDir()

	notifiers := []NotifierInfo{
		{
			Name: "Bark",
			Fields: []FieldInfo{
				{Name: "ServerURL", Key: "server_url", Title: "Server URL"},
				{Name: "DeviceKey", Key: "device_key", Title: "Device Key"},
			},
		},
		{
			Name: "Gotify",
			Fields: []FieldInfo{
				// Duplicated across notifiers: must appear only once.
				{Name: "ServerURL", Key: "server_url", Title: "Server URL"},
				{Name: "Token", Key: "token", Title: "Token"},
			},
		},
	}

	if err := generateLabelsFile(notifiers, dir); err != nil {
		t.Fatalf("generateLabelsFile() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "labels.ts"))
	if err != nil {
		t.Fatalf("reading labels.ts: %v", err)
	}

	got := string(content)
	want := `export const externalNotifyLabels = [
  $gettext('Device Key'),
  $gettext('Server URL'),
  $gettext('Token'),
]
`
	if !strings.Contains(got, want) {
		t.Errorf("labels.ts does not contain the expected sorted, deduplicated list.\ngot:\n%s\nwant to contain:\n%s", got, want)
	}
}

// TestGenerateLabelsFileEscapesQuotes ensures an apostrophe in a title cannot
// break out of the single-quoted TypeScript literal, which would make the whole
// file unparsable and silently drop every msgid in it.
func TestGenerateLabelsFileEscapesQuotes(t *testing.T) {
	dir := t.TempDir()

	notifiers := []NotifierInfo{
		{
			Name:   "Example",
			Fields: []FieldInfo{{Name: "Owner", Key: "owner", Title: "Owner's Token"}},
		},
	}

	if err := generateLabelsFile(notifiers, dir); err != nil {
		t.Fatalf("generateLabelsFile() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "labels.ts"))
	if err != nil {
		t.Fatalf("reading labels.ts: %v", err)
	}

	if want := `$gettext('Owner\'s Token')`; !strings.Contains(string(content), want) {
		t.Errorf("labels.ts = %q, want it to contain %q", string(content), want)
	}
}

// TestGenerateLabelsFileSkipsEmptyTitles guards against emitting an empty
// msgid, which gettext reserves for the catalog header.
func TestGenerateLabelsFileSkipsEmptyTitles(t *testing.T) {
	dir := t.TempDir()

	notifiers := []NotifierInfo{
		{
			Name: "Example",
			Fields: []FieldInfo{
				{Name: "Blank", Key: "blank", Title: ""},
				{Name: "Token", Key: "token", Title: "Token"},
			},
		},
	}

	if err := generateLabelsFile(notifiers, dir); err != nil {
		t.Fatalf("generateLabelsFile() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "labels.ts"))
	if err != nil {
		t.Fatalf("reading labels.ts: %v", err)
	}

	if strings.Contains(string(content), "$gettext('')") {
		t.Errorf("labels.ts emitted an empty msgid:\n%s", string(content))
	}
}
