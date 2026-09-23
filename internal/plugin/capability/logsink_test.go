package capability

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/sink"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEntriesMapTheParsedFields(t *testing.T) {
	at := time.Date(2026, 9, 23, 8, 15, 2, 0, time.FixedZone("JST", 9*3600))
	out := LogEntries([]sink.Entry{{
		LogPath: "/var/log/nginx/access.log", Time: at, Parsed: true,
		RemoteAddr: "203.0.113.7", Method: "GET", URI: "/a?b=c", Protocol: "HTTP/1.1", Status: 200,
		BodyBytesSent: 6 << 30, Referer: "https://example.com/", UserAgent: "curl/8.9.1",
		RequestTime: 0.004, UpstreamResponseTime: 0.003, Raw: "raw line",
	}, {
		LogPath: "/var/log/nginx/access.log", Time: at, Raw: "garbage", RemoteAddr: "ignored",
	}, {
		LogPath: "/var/log/nginx/access.log", Raw: "no time",
	}})
	require.Len(t, out, 3)

	assert.Equal(t, protocol.LogSinkPushParams{
		LogPath: "/var/log/nginx/access.log",
		Entry: protocol.LogEntry{
			Timestamp: "2026-09-22T23:15:02Z", RemoteAddr: "203.0.113.7", RequestMethod: "GET", RequestURI: "/a?b=c",
			Protocol: "HTTP/1.1", Status: 200, BodyBytesSent: 6 << 30, Referer: "https://example.com/",
			UserAgent: "curl/8.9.1", RequestTime: 0.004, UpstreamResponseTime: 0.003, Raw: "raw line",
			Format: protocol.LogFormatCombined,
		},
	}, out[0])
	assert.Equal(t, protocol.LogEntry{Timestamp: "2026-09-22T23:15:02Z", Raw: "garbage", Format: protocol.LogFormatRaw}, out[1].Entry,
		"a raw line carries only raw and timestamp")
	assert.Empty(t, out[2].Entry.Timestamp)
}

func TestLogFeedDeliversTheLinesOfTheHub(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")
	require.NoError(t, os.WriteFile(path, []byte("old line\n"), 0o644))
	hub := sink.NewHub(sink.Source{Paths: func() []string { return []string{path} }},
		sink.Config{PollInterval: 5 * time.Millisecond})
	feed := NewLogFeed(hub)

	var mu sync.Mutex
	var got []protocol.LogSinkPushParams
	unsubscribe := feed.Subscribe(func(entries []protocol.LogSinkPushParams) {
		mu.Lock()
		got = append(got, entries...)
		mu.Unlock()
	})
	defer unsubscribe()
	assert.True(t, hub.Active())

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(`203.0.113.7 - - [23/Sep/2026:08:15:02 +0000] "GET / HTTP/1.1" 204 0 "-" "curl/8.9.1"` + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 1
	}, 5*time.Second, 5*time.Millisecond)
	mu.Lock()
	entry := got[0]
	mu.Unlock()
	assert.Equal(t, path, entry.LogPath)
	assert.Equal(t, protocol.LogFormatCombined, entry.Entry.Format)
	assert.Equal(t, 204, entry.Entry.Status)
	assert.Equal(t, "2026-09-23T08:15:02Z", entry.Entry.Timestamp)

	unsubscribe()
	assert.False(t, hub.Active())
}
