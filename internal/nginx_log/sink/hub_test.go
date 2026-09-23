package sink

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	lineA = `203.0.113.7 - - [23/Sep/2026:08:15:02 +0000] "GET /index.html?lang=en HTTP/1.1" 200 612 "https://example.com/" "curl/8.9.1" 0.004 0.003`
	lineB = `198.51.100.2 - - [23/Sep/2026:08:15:03 +0000] "POST /api/login HTTP/2.0" 401 17 "-" "-"`
	lineC = `192.0.2.9 - - [23/Sep/2026:08:15:04 +0000] "GET /old HTTP/1.1" 301 0 "-" "Mozilla/5.0"`
)

// collector keeps every entry it was handed.
type collector struct {
	mu      sync.Mutex
	entries []Entry
}

func (c *collector) Deliver(entries []Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entries...)
}

func (c *collector) raws() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	raws := make([]string, 0, len(c.entries))
	for _, e := range c.entries {
		raws = append(raws, e.Raw)
	}
	return raws
}

func (c *collector) waitFor(t *testing.T, n int) []string {
	t.Helper()
	require.Eventually(t, func() bool { return len(c.raws()) >= n }, 5*time.Second, 5*time.Millisecond,
		"want %d lines, got %v", n, c.raws())
	return c.raws()
}

func fastConfig() Config {
	return Config{PollInterval: 5 * time.Millisecond, RefreshInterval: 10 * time.Millisecond}
}

func appendLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	for _, line := range lines {
		_, err = f.WriteString(line + "\n")
		require.NoError(t, err)
	}
}

func staticSource(paths ...string) Source {
	return Source{Paths: func() []string { return paths }}
}

// subscribe starts h with c. The feeder has its files open on return.
func subscribe(t *testing.T, h *Hub, c *collector) func() {
	t.Helper()
	unsubscribe := h.Subscribe(c)
	t.Cleanup(unsubscribe)
	return unsubscribe
}

func TestFeederDeliversOnlyNewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	appendLines(t, path, lineC)

	h := NewHub(staticSource(path), fastConfig())
	assert.False(t, h.Active())
	c := &collector{}
	subscribe(t, h, c)
	assert.True(t, h.Active())

	appendLines(t, path, lineA, "not an access log line at all", lineB)
	raws := c.waitFor(t, 3)
	assert.Equal(t, []string{lineA, "not an access log line at all", lineB}, raws, "history before the start is not replayed")

	c.mu.Lock()
	parsed, raw, second := c.entries[0], c.entries[1], c.entries[2]
	c.mu.Unlock()

	assert.True(t, parsed.Parsed)
	assert.Equal(t, path, parsed.LogPath)
	assert.Equal(t, time.Date(2026, 9, 23, 8, 15, 2, 0, time.UTC), parsed.Time)
	assert.Equal(t, "203.0.113.7", parsed.RemoteAddr)
	assert.Equal(t, "GET", parsed.Method)
	assert.Equal(t, "/index.html?lang=en", parsed.URI)
	assert.Equal(t, "HTTP/1.1", parsed.Protocol)
	assert.Equal(t, 200, parsed.Status)
	assert.EqualValues(t, 612, parsed.BodyBytesSent)
	assert.Equal(t, "https://example.com/", parsed.Referer)
	assert.Equal(t, "curl/8.9.1", parsed.UserAgent)
	assert.InDelta(t, 0.004, parsed.RequestTime, 1e-9)
	assert.InDelta(t, 0.003, parsed.UpstreamResponseTime, 1e-9)

	assert.False(t, raw.Parsed)
	assert.Empty(t, raw.RemoteAddr)
	assert.False(t, raw.Time.IsZero())

	assert.Empty(t, second.Referer, "a dash is an empty value")
	assert.Empty(t, second.UserAgent)
	assert.Equal(t, 401, second.Status)
}

func TestFeederFollowsRotationWithoutDuplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	appendLines(t, path, lineC)

	h := NewHub(staticSource(path), fastConfig())
	c := &collector{}
	subscribe(t, h, c)

	appendLines(t, path, lineA)
	c.waitFor(t, 1)

	// logrotate renames the file; nginx writes one more line to the old
	// inode before it reopens the log at the old path.
	rotated := path + ".1"
	require.NoError(t, os.Rename(path, rotated))
	appendLines(t, rotated, lineB)
	c.waitFor(t, 2)
	appendLines(t, path, lineC, lineA)

	raws := c.waitFor(t, 4)
	assert.Equal(t, []string{lineA, lineB, lineC, lineA}, raws)

	// Truncating in place restarts at the first byte.
	require.NoError(t, os.Truncate(path, 0))
	appendLines(t, path, lineB)
	raws = c.waitFor(t, 5)
	assert.Equal(t, lineB, raws[4])
	time.Sleep(30 * time.Millisecond)
	assert.Len(t, c.raws(), 5, "nothing is delivered twice")
}

func TestFeederReadsAFileCreatedLaterFromItsStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")

	h := NewHub(staticSource(path), fastConfig())
	c := &collector{}
	subscribe(t, h, c)

	appendLines(t, path, lineA, lineB)
	assert.Equal(t, []string{lineA, lineB}, c.waitFor(t, 2))
}

func TestFeederJoinsPartialLinesAndDropsLongOnes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	appendLines(t, path)

	cfg := fastConfig()
	cfg.MaxLineLength = 200
	h := NewHub(staticSource(path), cfg)
	c := &collector{}
	subscribe(t, h, c)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.WriteString(lineA[:40])
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, c.raws(), "an unfinished line waits for its break")

	_, err = f.WriteString(lineA[40:] + "\r\n" + strings.Repeat("x", 300))
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	_, err = f.WriteString(strings.Repeat("y", 50) + "\n" + lineB + "\n")
	require.NoError(t, err)

	assert.Equal(t, []string{lineA, lineB}, c.waitFor(t, 2))
	time.Sleep(20 * time.Millisecond)
	assert.Len(t, c.raws(), 2, "the overlong line is dropped whole")
}

func TestFeederHonoursTheSource(t *testing.T) {
	dir := t.TempDir()
	allowed := filepath.Join(dir, "allowed.log")
	denied := filepath.Join(dir, "denied.log")
	compressed := filepath.Join(dir, "old.log.gz")
	for _, path := range []string{allowed, denied, compressed} {
		appendLines(t, path)
	}

	h := NewHub(Source{
		Paths:   func() []string { return []string{allowed, denied, compressed, allowed} },
		Allowed: func(path string) bool { return path != denied },
	}, fastConfig())
	c := &collector{}
	subscribe(t, h, c)

	appendLines(t, denied, lineB)
	appendLines(t, compressed, lineC)
	appendLines(t, allowed, lineA)
	assert.Equal(t, []string{lineA}, c.waitFor(t, 1))
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, []string{lineA}, c.raws())

	// A path the source drops is no longer followed.
	h.SetSource(staticSource(denied))
	time.Sleep(40 * time.Millisecond)
	appendLines(t, allowed, lineB)
	appendLines(t, denied, lineC)
	raws := c.waitFor(t, 2)
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, []string{lineA, lineC}, c.raws(), "got %v", raws)
}

func TestFeederRunsOnlyWithSubscribers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	appendLines(t, path)

	h := NewHub(staticSource(path), fastConfig())
	first, second := &collector{}, &collector{}
	stopFirst := h.Subscribe(first)
	stopSecond := h.Subscribe(second)

	appendLines(t, path, lineA)
	first.waitFor(t, 1)
	second.waitFor(t, 1)

	stopFirst()
	stopFirst()
	assert.True(t, h.Active())
	appendLines(t, path, lineB)
	second.waitFor(t, 2)
	assert.Len(t, first.raws(), 1, "an unsubscribed collector gets nothing")

	stopSecond()
	assert.False(t, h.Active())
	h.mu.Lock()
	running := h.cancel != nil
	h.mu.Unlock()
	assert.False(t, running, "the feeder stops with the last subscriber")

	// A new subscriber starts at the end again.
	appendLines(t, path, lineC)
	third := &collector{}
	subscribe(t, h, third)
	appendLines(t, path, lineA)
	assert.Equal(t, []string{lineA}, third.waitFor(t, 1))
}

func TestDefaultHubUsesTheInstalledSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	appendLines(t, path)

	previous := Default().currentSource()
	t.Cleanup(func() { SetSource(previous) })
	SetSource(staticSource(path))

	c := &collector{}
	unsubscribe := Subscribe(SubscriberFunc(c.Deliver))
	defer unsubscribe()
	appendLines(t, path, lineA)
	require.Eventually(t, func() bool { return len(c.raws()) == 1 }, 10*time.Second, 20*time.Millisecond)
}
