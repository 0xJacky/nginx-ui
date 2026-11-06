package geolite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticAvailability Availability

func (s staticAvailability) Availability() Availability { return Availability(s) }

func installAvailabilityProvider(t *testing.T, p AvailabilityProvider) {
	t.Helper()
	original := availabilityProvider
	SetAvailabilityProvider(p)
	t.Cleanup(func() { SetAvailabilityProvider(original) })
}

func TestCurrentAvailabilityWithoutDatabase(t *testing.T) {
	installAvailabilityProvider(t, nil)
	dir := useTempConfDir(t)

	assert.Equal(t, Availability{Path: filepath.Join(dir, "GeoLite2-City.mmdb")}, CurrentAvailability())
}

func TestCurrentAvailabilityReportsFileDetails(t *testing.T) {
	installAvailabilityProvider(t, nil)
	dir := useTempConfDir(t)

	path := filepath.Join(dir, "GeoLite2-City.mmdb")
	content := []byte("twelve bytes")
	require.NoError(t, os.WriteFile(path, content, 0o644))
	modified := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.Chtimes(path, modified, modified))

	got := CurrentAvailability()

	assert.True(t, got.Exists)
	assert.Equal(t, path, got.Path)
	assert.Equal(t, int64(len(content)), got.Size)
	assert.True(t, modified.Equal(got.LastModified), "got %v, want %v", got.LastModified, modified)
}

func TestCurrentAvailabilityFollowsCustomPath(t *testing.T) {
	installAvailabilityProvider(t, nil)
	useTempConfDir(t)

	custom := filepath.Join(t.TempDir(), "enterprise.mmdb")
	require.NoError(t, os.WriteFile(custom, []byte("db"), 0o644))
	setCustomMMDB(t, custom)

	got := CurrentAvailability()
	assert.True(t, got.Exists)
	assert.Equal(t, custom, got.Path)
	assert.Equal(t, int64(2), got.Size)
}

func TestCurrentAvailabilityHonoursProvider(t *testing.T) {
	useTempConfDir(t)
	want := Availability{
		Exists:       true,
		Path:         "/demo/GeoLite2-City.mmdb",
		Size:         61 << 20,
		LastModified: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	installAvailabilityProvider(t, staticAvailability(want))

	// The override wins even though nothing exists on disk.
	assert.False(t, DBExists())
	assert.Equal(t, want, CurrentAvailability())
}

func TestSetAvailabilityProviderNilRestoresFilesystemCheck(t *testing.T) {
	useTempConfDir(t)
	installAvailabilityProvider(t, staticAvailability{Exists: true, Path: "/demo"})

	SetAvailabilityProvider(nil)

	got := CurrentAvailability()
	assert.False(t, got.Exists)
	assert.Equal(t, GetDBPath(), got.Path)
}
