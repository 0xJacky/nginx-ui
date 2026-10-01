package snippet

import (
	"fmt"
	"path/filepath"
	"testing"
)

// writeSites writes 20 snippets and 2000 sites that include them, and feeds
// them to the scanner index when indexed is set.
func writeSites(b *testing.B, indexed bool) {
	confDir := setupSnippetTestB(b)
	if indexed {
		usage.Lock()
		usage.includes, usage.headers, usage.ready = map[string][]string{}, map[string]header{}, true
		usage.Unlock()
		b.Cleanup(func() {
			usage.Lock()
			usage.includes, usage.headers, usage.ready = map[string][]string{}, map[string]header{}, false
			usage.Unlock()
		})
	}
	write := func(path, content string) {
		writeFileB(b, path, content)
		if indexed {
			_ = scanIncludes(path, []byte(content))
		}
	}
	for i := range 20 {
		write(filepath.Join(confDir, "snippets", fmt.Sprintf("s%02d.conf", i)), "# x\nexpires 7d;\n")
	}
	for i := range 2000 {
		write(filepath.Join(confDir, "sites-available", fmt.Sprintf("site%04d.example.net", i)),
			fmt.Sprintf("server {\n    listen 80;\n    server_name site%04d.example.net;\n    location /a/ {\n        include snippets/s%02d.conf;\n    }\n    location /b/ { proxy_pass http://127.0.0.1:8080; proxy_set_header Host $host; }\n}\n", i, i%20))
	}
}

// BenchmarkListScanning lists while the scanner is not ready, reading every
// file.
func BenchmarkListScanning(b *testing.B) {
	writeSites(b, false)
	b.ResetTimer()
	for b.Loop() {
		if _, err := List(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListIndexed lists from the scanner index.
func BenchmarkListIndexed(b *testing.B) {
	writeSites(b, true)
	b.ResetTimer()
	for b.Loop() {
		if _, err := List(); err != nil {
			b.Fatal(err)
		}
	}
}
