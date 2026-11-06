package geolite

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	nginxSettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

func TestGetDBXZPath(t *testing.T) {
	dir := useTempConfDir(t)

	assert.Equal(t, filepath.Join(dir, "GeoLite2-City.mmdb.xz"), GetDBXZPath())
}

func TestGetDBPath(t *testing.T) {
	absoluteCustom := filepath.Join(t.TempDir(), "enterprise.mmdb")

	tests := []struct {
		name          string
		custom        string
		defaultExists bool
		want          func(confDir string) string
	}{
		{
			name: "default path when nothing is configured",
			want: func(confDir string) string { return filepath.Join(confDir, "GeoLite2-City.mmdb") },
		},
		{
			name:   "blank custom path is ignored",
			custom: "   ",
			want:   func(confDir string) string { return filepath.Join(confDir, "GeoLite2-City.mmdb") },
		},
		{
			name:   "absolute custom path",
			custom: absoluteCustom,
			want:   func(string) string { return absoluteCustom },
		},
		{
			name:   "relative custom path resolves against the config directory",
			custom: " geo/city.mmdb ",
			want:   func(confDir string) string { return filepath.Join(confDir, "geo", "city.mmdb") },
		},
		{
			name:          "existing default database wins over a custom path",
			custom:        absoluteCustom,
			defaultExists: true,
			want:          func(confDir string) string { return filepath.Join(confDir, "GeoLite2-City.mmdb") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := useTempConfDir(t)
			setCustomMMDB(t, tt.custom)
			if tt.defaultExists {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "GeoLite2-City.mmdb"), []byte("db"), 0o644))
			}

			assert.Equal(t, tt.want(dir), GetDBPath())
		})
	}
}

func TestDBExists(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		useTempConfDir(t)
		assert.False(t, DBExists())
	})

	t.Run("default database present", func(t *testing.T) {
		dir := useTempConfDir(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "GeoLite2-City.mmdb"), []byte("db"), 0o644))
		assert.True(t, DBExists())
	})

	t.Run("custom database present", func(t *testing.T) {
		useTempConfDir(t)
		custom := filepath.Join(t.TempDir(), "city.mmdb")
		require.NoError(t, os.WriteFile(custom, []byte("db"), 0o644))
		setCustomMMDB(t, custom)
		assert.True(t, DBExists())
	})

	t.Run("custom database configured but missing", func(t *testing.T) {
		useTempConfDir(t)
		setCustomMMDB(t, filepath.Join(t.TempDir(), "missing.mmdb"))
		assert.False(t, DBExists())
	})
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func drain(ch chan float64) []float64 {
	var values []float64
	for {
		select {
		case v := <-ch:
			values = append(values, v)
		default:
			return values
		}
	}
}

func TestDownloadProgressWriter(t *testing.T) {
	t.Run("reports when progress crosses the interval", func(t *testing.T) {
		var out bytes.Buffer
		progress := make(chan float64, 16)
		w := &DownloadProgressWriter{Writer: &out, totalSize: 100, progressChan: progress, reportInterval: 10}

		for _, size := range []int{5, 4, 1, 30, 60} {
			n, err := w.Write(make([]byte, size))
			require.NoError(t, err)
			require.Equal(t, size, n)
		}

		assert.Equal(t, 100, out.Len())
		assert.Equal(t, []float64{10, 40, 100}, drain(progress))
	})

	t.Run("does not block on a full channel and retries later", func(t *testing.T) {
		progress := make(chan float64) // unbuffered and never read
		w := &DownloadProgressWriter{Writer: &bytes.Buffer{}, totalSize: 4, progressChan: progress, reportInterval: 1}

		n, err := w.Write([]byte("ab"))
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		// The dropped report leaves lastReported unchanged, so the next write
		// reports the accumulated progress once there is room.
		buffered := make(chan float64, 1)
		w.progressChan = buffered
		_, err = w.Write([]byte("c"))
		require.NoError(t, err)
		assert.Equal(t, []float64{75}, drain(buffered))
	})

	t.Run("propagates write errors", func(t *testing.T) {
		wantErr := errors.New("disk full")
		progress := make(chan float64, 1)
		w := &DownloadProgressWriter{Writer: failingWriter{err: wantErr}, totalSize: 10, progressChan: progress, reportInterval: 1}

		n, err := w.Write([]byte("data"))
		assert.ErrorIs(t, err, wantErr)
		assert.Zero(t, n)
		assert.Empty(t, drain(progress))
	})
}

// useDownloadProxy routes DownloadGeoLiteDB through handler by configuring it
// as the HTTP proxy, so the hard-coded download URL never leaves the machine.
func useDownloadProxy(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	setHTTPProxy(t, server.URL)
}

func setHTTPProxy(t *testing.T, proxy string) {
	t.Helper()
	original := nginxSettings.HTTPSettings.HTTPProxy
	nginxSettings.HTTPSettings.HTTPProxy = proxy
	t.Cleanup(func() { nginxSettings.HTTPSettings.HTTPProxy = original })
}

func TestDownloadGeoLiteDB(t *testing.T) {
	payload := bytes.Repeat([]byte("geolite-payload-"), 4096)

	t.Run("saves the archive and reports progress", func(t *testing.T) {
		useTempConfDir(t)
		requested := make(chan string, 1)
		useDownloadProxy(t, func(w http.ResponseWriter, r *http.Request) {
			requested <- r.Method + " " + r.URL.String()
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			_, _ = w.Write(payload)
		})

		progress := make(chan float64, 256)
		require.NoError(t, DownloadGeoLiteDB(progress))

		assert.Equal(t, http.MethodGet+" "+DownloadURL, <-requested)

		saved, err := os.ReadFile(GetDBXZPath())
		require.NoError(t, err)
		assert.Equal(t, payload, saved)

		values := drain(progress)
		require.NotEmpty(t, values)
		assert.IsNonDecreasing(t, values)
		assert.Equal(t, float64(100), values[len(values)-1])
	})

	t.Run("non-200 status", func(t *testing.T) {
		useTempConfDir(t)
		useDownloadProxy(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "gone", http.StatusNotFound)
		})

		err := DownloadGeoLiteDB(make(chan float64, 1))
		got := requireCosyError(t, err, ErrDownloadFailed)
		assert.Contains(t, got.Error(), "status code: 404")
		assert.NoFileExists(t, GetDBXZPath())
	})

	t.Run("missing content length", func(t *testing.T) {
		useTempConfDir(t)
		useDownloadProxy(t, func(w http.ResponseWriter, r *http.Request) {
			// Flushing before writing forces chunked encoding without a length.
			w.(http.Flusher).Flush()
			_, _ = w.Write(payload)
		})

		err := DownloadGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrFailedToGetFileSize)
		assert.NoFileExists(t, GetDBXZPath())
	})

	t.Run("connection drops mid-download", func(t *testing.T) {
		useTempConfDir(t)
		useDownloadProxy(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			_, _ = w.Write(payload[:len(payload)/2])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		})

		err := DownloadGeoLiteDB(make(chan float64, 256))
		requireCosyError(t, err, ErrFailedToSaveFile)
		assert.NoFileExists(t, GetDBXZPath(), "a partial archive should be removed")
	})

	t.Run("config directory missing", func(t *testing.T) {
		dir := useTempConfDir(t)
		useDownloadProxy(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			_, _ = w.Write(payload)
		})
		require.NoError(t, os.Remove(dir))

		err := DownloadGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrFailedToCreateFile)
	})

	t.Run("invalid proxy URL", func(t *testing.T) {
		useTempConfDir(t)
		setHTTPProxy(t, "http://[::1]:namedport")

		err := DownloadGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrDownloadFailed)
	})

	t.Run("proxy unreachable", func(t *testing.T) {
		useTempConfDir(t)
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		setHTTPProxy(t, server.URL)

		err := DownloadGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrDownloadFailed)
	})
}

func xzCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// decompressPayload is large enough to span several read buffers.
func decompressPayload() []byte {
	var buf bytes.Buffer
	for i := range 20000 {
		fmt.Fprintf(&buf, "%d,", i)
	}
	return buf.Bytes()
}

func TestDecompressGeoLiteDB(t *testing.T) {
	payload := decompressPayload()
	compressed := xzCompress(t, payload)

	t.Run("writes the database and removes the archive", func(t *testing.T) {
		useTempConfDir(t)
		require.NoError(t, os.WriteFile(GetDBXZPath(), compressed, 0o644))

		progress := make(chan float64, 1024)
		require.NoError(t, DecompressGeoLiteDB(progress))

		got, err := os.ReadFile(GetDBPath())
		require.NoError(t, err)
		assert.Equal(t, payload, got)
		assert.NoFileExists(t, GetDBXZPath())
		assert.True(t, DBExists())

		values := drain(progress)
		require.NotEmpty(t, values)
		assert.IsNonDecreasing(t, values)
		assert.Equal(t, float64(100), values[len(values)-1])
		for _, v := range values[:len(values)-1] {
			assert.LessOrEqual(t, v, float64(99), "progress is capped below 100 until completion")
		}
	})

	t.Run("does not block when nobody reads progress", func(t *testing.T) {
		useTempConfDir(t)
		require.NoError(t, os.WriteFile(GetDBXZPath(), compressed, 0o644))

		require.NoError(t, DecompressGeoLiteDB(make(chan float64)))

		got, err := os.ReadFile(GetDBPath())
		require.NoError(t, err)
		assert.Equal(t, payload, got)
	})

	t.Run("leaves a custom database untouched", func(t *testing.T) {
		confDir := useTempConfDir(t)
		customPath := filepath.Join(t.TempDir(), "enterprise.mmdb")
		customContent := []byte("custom enterprise database")
		require.NoError(t, os.WriteFile(customPath, customContent, 0o644))
		setCustomMMDB(t, customPath)
		require.Equal(t, customPath, GetDBPath(), "the custom database is in use before the download")
		require.NoError(t, os.WriteFile(GetDBXZPath(), compressed, 0o644))

		require.NoError(t, DecompressGeoLiteDB(make(chan float64, 1024)))

		got, err := os.ReadFile(customPath)
		require.NoError(t, err)
		assert.Equal(t, customContent, got, "the download must not overwrite the custom database")

		defaultPath := filepath.Join(confDir, "GeoLite2-City.mmdb")
		got, err = os.ReadFile(defaultPath)
		require.NoError(t, err)
		assert.Equal(t, payload, got)
	})

	t.Run("archive missing", func(t *testing.T) {
		useTempConfDir(t)

		err := DecompressGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrFailedToOpenFile)
		assert.False(t, DBExists())
	})

	t.Run("archive is not xz", func(t *testing.T) {
		useTempConfDir(t)
		require.NoError(t, os.WriteFile(GetDBXZPath(), []byte("definitely not xz"), 0o644))

		err := DecompressGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrFailedToCreateXZReader)
		assert.False(t, DBExists())
		assert.FileExists(t, GetDBXZPath())
	})

	t.Run("archive truncated", func(t *testing.T) {
		useTempConfDir(t)
		require.NoError(t, os.WriteFile(GetDBXZPath(), compressed[:len(compressed)/2], 0o644))

		err := DecompressGeoLiteDB(make(chan float64, 64))
		requireCosyError(t, err, ErrFailedToReadData)
		assert.False(t, DBExists(), "a partial database should be removed")
		assert.FileExists(t, GetDBXZPath())
	})

	t.Run("database path not writable", func(t *testing.T) {
		dir := useTempConfDir(t)
		require.NoError(t, os.WriteFile(GetDBXZPath(), compressed, 0o644))
		// A directory at the database path makes os.Create fail.
		require.NoError(t, os.Mkdir(filepath.Join(dir, "GeoLite2-City.mmdb"), 0o755))

		err := DecompressGeoLiteDB(make(chan float64, 1))
		requireCosyError(t, err, ErrFailedToCreateFile)
		assert.FileExists(t, GetDBXZPath())
	})
}
