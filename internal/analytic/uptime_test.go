package analytic

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadUptime(t *testing.T) {
	const fallbackUptime = 999999
	fallback := func() (uint64, error) { return fallbackUptime, nil }
	tests := []struct {
		name    string
		goos    string
		content *string
		want    uint64
	}{
		{"container uptime from proc", "linux", new("7932.15 31234.56\n"), 7932},
		{"missing file", "linux", nil, fallbackUptime},
		{"empty file", "linux", new(""), fallbackUptime},
		{"malformed value", "linux", new("abc 31234.56\n"), fallbackUptime},
		{"negative value", "linux", new("-1 0\n"), fallbackUptime},
		{"non-linux ignores proc", "darwin", new("7932.15 31234.56\n"), fallbackUptime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "uptime")
			if tt.content != nil {
				if err := os.WriteFile(path, []byte(*tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := readUptime(tt.goos, path, fallback)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d; want %d", got, tt.want)
			}
		})
	}
}

func TestReadUptimeFallbackError(t *testing.T) {
	fallbackErr := errors.New("sysinfo unavailable")
	_, err := readUptime("linux", filepath.Join(t.TempDir(), "missing"), func() (uint64, error) {
		return 0, fallbackErr
	})
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("got %v; want %v", err, fallbackErr)
	}
}
