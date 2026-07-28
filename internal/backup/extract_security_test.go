package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

type testZipEntry struct {
	name    string
	content string
	mode    os.FileMode
}

func writeTestZip(t *testing.T, entries []testZipEntry) string {
	t.Helper()
	zipPath := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		header.SetMode(entry.mode)
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entryWriter.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestExtractZipArchiveRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{
		"../outside",
		"directory/../../outside",
		"/absolute/path",
		"C:/windows/path",
		`directory\windows-path`,
	} {
		t.Run(name, func(t *testing.T) {
			zipPath := writeTestZip(t, []testZipEntry{{name: name, content: "pwned", mode: 0o644}})
			destination := t.TempDir()
			if err := extractZipArchive(zipPath, destination); err == nil {
				t.Fatalf("extractZipArchive accepted unsafe path %q", name)
			}
		})
	}
}

func TestExtractZipArchiveRejectsDuplicateAndConflictingEntries(t *testing.T) {
	tests := map[string][]testZipEntry{
		"duplicate": {
			{name: "same", content: "one", mode: 0o644},
			{name: "same", content: "two", mode: 0o644},
		},
		"file parent": {
			{name: "parent", content: "file", mode: 0o644},
			{name: "parent/child", content: "child", mode: 0o644},
		},
		"symlink parent": {
			{name: "parent", content: "target", mode: os.ModeSymlink | 0o777},
			{name: "parent/child", content: "child", mode: 0o644},
		},
	}

	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			zipPath := writeTestZip(t, entries)
			if err := extractZipArchive(zipPath, t.TempDir()); err == nil {
				t.Fatal("extractZipArchive accepted conflicting entries")
			}
		})
	}
}

func TestExtractZipArchiveDoesNotWriteThroughSymlink(t *testing.T) {
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "child")
	zipPath := writeTestZip(t, []testZipEntry{
		{name: "parent", content: outside, mode: os.ModeSymlink | 0o777},
		{name: "parent/child", content: "pwned", mode: 0o644},
	})

	if err := extractZipArchive(zipPath, t.TempDir()); err == nil {
		t.Fatal("extractZipArchive accepted a symlink ancestor")
	}
	if _, err := os.Stat(outsideFile); !os.IsNotExist(err) {
		t.Fatalf("archive wrote outside extraction root: %v", err)
	}
}

func TestExtractZipArchivePreservesSafeRelativeSymlink(t *testing.T) {
	zipPath := writeTestZip(t, []testZipEntry{
		{name: "available/site.conf", content: "server {}", mode: 0o644},
		{name: "enabled/site.conf", content: "../available/site.conf", mode: os.ModeSymlink | 0o777},
	})
	destination := t.TempDir()

	if err := extractZipArchive(zipPath, destination); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(destination, "enabled", "site.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join("..", "available", "site.conf") {
		t.Fatalf("unexpected symlink target %q", target)
	}
}

func TestExtractNginxZipArchiveRewritesAbsoluteConfigSymlink(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), "nginx")
	absTarget := filepath.Join(configRoot, "sites-available", "site.conf")
	zipPath := writeTestZip(t, []testZipEntry{
		{name: "sites-available/site.conf", content: "server {}", mode: 0o644},
		{name: "sites-enabled/site.conf", content: absTarget, mode: os.ModeSymlink | 0o777},
	})
	destination := t.TempDir()

	if err := extractNginxZipArchive(zipPath, destination, configRoot, ""); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(destination, "sites-enabled", "site.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join("..", "sites-available", "site.conf") {
		t.Fatalf("absolute config symlink was not rewritten safely: %q", target)
	}
}

func TestExtractZipArchiveRequiresEmptyDestination(t *testing.T) {
	destination := t.TempDir()
	if err := os.WriteFile(filepath.Join(destination, "existing"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := writeTestZip(t, []testZipEntry{{name: "new", content: "data", mode: 0o644}})

	if err := extractZipArchive(zipPath, destination); err == nil {
		t.Fatal("extractZipArchive accepted a non-empty destination")
	}
	data, err := os.ReadFile(filepath.Join(destination, "existing"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("existing destination content changed: %q, %v", data, err)
	}
}
