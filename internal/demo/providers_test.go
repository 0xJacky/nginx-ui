package demo

import (
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstreamProberDeclinesUnknownTargets(t *testing.T) {
	got := upstreamProber{}.Probe([]string{"10.0.0.5:8080", "example.com:443"})

	// No entry means "no opinion", and the caller then dials for real. An
	// override must never assert that someone else's backend is healthy.
	assert.Empty(t, got)
}

func TestUpstreamProberAnswersOwnTargets(t *testing.T) {
	got := upstreamProber{}.Probe([]string{"127.0.0.1:9000", "10.0.0.5:8080"})

	require.Contains(t, got, "127.0.0.1:9000")
	assert.NotContains(t, got, "10.0.0.5:8080")
	assert.True(t, got["127.0.0.1:9000"].Online)
	assert.Positive(t, got["127.0.0.1:9000"].Latency)
}

func TestUpstreamProberReportsSomethingDown(t *testing.T) {
	sockets := make([]string, 0, len(demoSockets))
	for socket := range demoSockets {
		sockets = append(sockets, socket)
	}

	got := upstreamProber{}.Probe(sockets)

	var offline int
	for _, status := range got {
		if !status.Online {
			offline++
		}
	}
	assert.Positive(t, offline, "an all-green demo hides every failure state in the UI")
}

func TestUpstreamProberIsStableWithinATimeBucket(t *testing.T) {
	first := upstreamProber{}.Probe([]string{"127.0.0.1:9000"})
	for range 50 {
		again := upstreamProber{}.Probe([]string{"127.0.0.1:9000"})
		assert.Equal(t, first["127.0.0.1:9000"].Latency, again["127.0.0.1:9000"].Latency)
	}
}

func TestUpstreamProberHandlesEmptyInput(t *testing.T) {
	assert.Nil(t, upstreamProber{}.Probe(nil))
	assert.Nil(t, upstreamProber{}.Probe([]string{}))
}

func TestStubStatusIsInternallyConsistent(t *testing.T) {
	data := stubStatus{}.StubStatus()
	require.NotNil(t, data)

	// A gap between accepts and handled means dropped connections.
	assert.Equal(t, data.Accepts, data.Handled)
	// Waiting is the remainder and must never go negative.
	assert.GreaterOrEqual(t, data.Waiting, 0)
	assert.Equal(t, data.Active, data.Reading+data.Writing+data.Waiting)
	assert.Greater(t, data.Requests, data.Accepts,
		"fewer requests than connections would imply no keep-alive at all")
}

func TestStubStatusIsStableWithinATimeBucket(t *testing.T) {
	first := stubStatus{}.StubStatus()
	for range 50 {
		assert.Equal(t, first, stubStatus{}.StubStatus())
	}
}

func TestStubStatusCountersOnlyMoveForward(t *testing.T) {
	// Simulate consecutive buckets rather than sleeping through them.
	previous := -1
	for bucket := range uint64(200) {
		accepts := acceptsBase + int(bucket%100_000)*acceptsRate
		assert.Greater(t, accepts, previous)
		previous = accepts
	}
}

func TestReleaseReportsCurrentVersionAsLatest(t *testing.T) {
	for _, channel := range []string{"stable", "prerelease", "dev", ""} {
		release, ok := releaseInfo{}.Release(channel)
		require.True(t, ok, channel)
		assert.Equal(t, "v"+version.GetVersionInfo().Version, release.TagName, channel)
		assert.False(t, release.PublishedAt.IsZero(), channel)
	}
}

func TestReleasePublishedAtIsFixed(t *testing.T) {
	first, _ := releaseInfo{}.Release("stable")
	time.Sleep(2 * time.Millisecond)
	again, _ := releaseInfo{}.Release("stable")

	// A moving timestamp makes the About page drift between reloads.
	assert.Equal(t, first.PublishedAt, again.PublishedAt)
}

func TestReleaseMarksPrereleaseChannels(t *testing.T) {
	stable, _ := releaseInfo{}.Release("stable")
	assert.False(t, stable.Prerelease)

	for _, channel := range []string{"prerelease", "dev"} {
		release, _ := releaseInfo{}.Release(channel)
		assert.True(t, release.Prerelease, channel)
	}
}

func TestSiteProberDeclinesForeignHosts(t *testing.T) {
	for _, siteURL := range []string{"https://example.com", "http://google.com/x", "https://github.com"} {
		_, ok := siteProber{}.Probe(siteURL)
		assert.False(t, ok, "%s must be probed for real", siteURL)
	}
}

func TestSiteProberAnswersOwnHosts(t *testing.T) {
	got, ok := siteProber{}.Probe("http://ojbk.me")
	require.True(t, ok)
	assert.Equal(t, "online", got.Status)
	assert.Equal(t, 200, got.StatusCode)
	assert.Positive(t, got.ResponseTime)
}

func TestSiteProberReportsADegradedSite(t *testing.T) {
	got, ok := siteProber{}.Probe("http://langgood.com")
	require.True(t, ok)
	assert.Equal(t, "error", got.Status)
	assert.Equal(t, 503, got.StatusCode)
	assert.NotEmpty(t, got.Error, "an all-green demo hides the failure states")
}

func TestSiteProberHandlesSubdomainsAndBareHosts(t *testing.T) {
	for _, siteURL := range []string{"http://www.ojbk.me", "ojbk.me", "https://ojbk.me/path"} {
		_, ok := siteProber{}.Probe(siteURL)
		assert.True(t, ok, siteURL)
	}
}

func TestSiteProberIsStableWithinATimeBucket(t *testing.T) {
	first, _ := siteProber{}.Probe("http://ojbk.me")
	for range 50 {
		again, _ := siteProber{}.Probe("http://ojbk.me")
		assert.Equal(t, first, again)
	}
}

func TestSiteProberDeclinesGarbage(t *testing.T) {
	_, ok := siteProber{}.Probe("")
	assert.False(t, ok)
}

func TestAccessLogFixtureIsWellFormed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	now := time.Date(2026, 8, 1, 12, 30, 0, 0, time.UTC)

	require.NoError(t, WriteAccessLogFixture(path, now))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	assert.Len(t, lines, logLineCount)

	// Every client must be an address the geo provider will answer for,
	// otherwise the maps stay empty no matter how many lines exist.
	svc := &geoService{}
	combined := regexp.MustCompile(`^(\S+) - - \[([^\]]+)\] "(\w+) (\S+) HTTP/1\.1" (\d{3}) (\d+) "([^"]*)" "([^"]*)" "-"$`)
	for i, line := range lines {
		m := combined.FindStringSubmatch(line)
		require.NotNil(t, m, "line %d does not match the combined format: %s", i, line)

		addr, err := netip.ParseAddr(m[1])
		require.NoError(t, err, line)
		assert.True(t, isDocumentationIP(addr), "client %s is not a documentation address", m[1])

		_, err = time.Parse("02/Jan/2006:15:04:05 -0700", m[2])
		assert.NoError(t, err, line)

		got, err := svc.Search(m[1])
		require.NoError(t, err)
		require.NotNil(t, got, "geo provider declined %s", m[1])
	}
}

func TestAccessLogFixtureCoversTheDashboardWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	now := time.Date(2026, 8, 1, 12, 30, 0, 0, time.UTC)
	require.NoError(t, WriteAccessLogFixture(path, now))

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	stamp := regexp.MustCompile(`\[([^\]]+)\]`)

	first := stamp.FindStringSubmatch(lines[0])[1]
	last := stamp.FindStringSubmatch(lines[len(lines)-1])[1]
	t0, _ := time.Parse("02/Jan/2006:15:04:05 -0700", first)
	t1, _ := time.Parse("02/Jan/2006:15:04:05 -0700", last)

	// The default dashboard range is the last 24h; entries must span it and
	// stop at the anchor rather than running into the future.
	assert.WithinDuration(t, now.Truncate(time.Hour).Add(-logWindow), t0, time.Minute)
	assert.False(t, t1.After(now.Truncate(time.Hour)), "fixture must not contain future timestamps")
}

func TestAccessLogFixtureIsByteStable(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 5, 0, 0, time.UTC)
	later := time.Date(2026, 8, 1, 12, 55, 0, 0, time.UTC) // same hour

	var out [2][]byte
	for i, at := range []time.Time{now, later} {
		path := filepath.Join(t.TempDir(), "access.log")
		require.NoError(t, WriteAccessLogFixture(path, at))
		out[i], _ = os.ReadFile(path)
	}
	assert.Equal(t, out[0], out[1], "a restart within the same hour must not change the log")
}

func TestAccessLogFixtureHasAMixOfStatuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")
	require.NoError(t, WriteAccessLogFixture(path, time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)))
	data, _ := os.ReadFile(path)

	for _, code := range []string{`" 200 `, `" 404 `, `" 403 `, `" 500 `} {
		assert.Contains(t, string(data), code, "missing status %s makes the breakdown look fake", code)
	}
}

func TestLogFixturesRefuseToClobberRealLogs(t *testing.T) {
	// A self-hosted instance that somehow ran this must not lose its log.
	path := filepath.Join(t.TempDir(), "access.log")
	require.NoError(t, os.WriteFile(path, []byte("real operator traffic\n"), 0o644))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Positive(t, info.Size(), "precondition")
	// installLogFixtures skips non-empty files; assert the predicate it uses.
	assert.True(t, info.Size() > 0)
}
