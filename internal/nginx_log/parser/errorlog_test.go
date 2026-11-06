// Ported from the official log analytics plugin (nginxui/plugin-log-analytics).

package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func localUnix(t *testing.T, value string) int64 {
	t.Helper()
	ts, err := time.ParseInLocation("2006/01/02 15:04:05", value, time.Local)
	require.NoError(t, err)
	return ts.Unix()
}

func TestParseErrorLineWithContext(t *testing.T) {
	line := `2026/09/07 15:32:29 [error] 1234#5678: *90 open() "/var/www/favicon.ico" failed (2: No such file or directory), client: 203.0.113.9, server: example.com, request: "GET /favicon.ico HTTP/1.1", upstream: "http://127.0.0.1:8080/favicon.ico", host: "example.com", referrer: "https://example.com/a, b"`
	e, ok := ParseErrorLine(line)
	require.True(t, ok)
	assert.Equal(t, localUnix(t, "2026/09/07 15:32:29"), e.Timestamp)
	assert.Equal(t, "error", e.Level)
	assert.Equal(t, int64(1234), e.PID)
	assert.Equal(t, int64(90), e.Connection)
	assert.Equal(t, `open() "/var/www/favicon.ico" failed (2: No such file or directory)`, e.Message)
	assert.Equal(t, "203.0.113.9", e.Client)
	assert.Equal(t, "example.com", e.Server)
	assert.Equal(t, "GET", e.Method)
	assert.Equal(t, "/favicon.ico", e.Path)
	assert.Equal(t, "http://127.0.0.1:8080/favicon.ico", e.Upstream)
	assert.Equal(t, "example.com", e.Host)
	assert.Equal(t, "https://example.com/a, b", e.Referrer)
}

func TestParseErrorLineWithoutConnectionOrContext(t *testing.T) {
	e, ok := ParseErrorLine("2026/09/07 15:32:29 [notice] 1#1: signal process started")
	require.True(t, ok)
	assert.Equal(t, "notice", e.Level)
	assert.Equal(t, int64(1), e.PID)
	assert.Zero(t, e.Connection)
	assert.Equal(t, "signal process started", e.Message)
	assert.Empty(t, e.Client)

	e, ok = ParseErrorLine("2026/09/07 15:32:29 [warn] 7#7: *3 an upstream response is buffered, client: ::1, server: _")
	require.True(t, ok)
	assert.Equal(t, "::1", e.Client)
	assert.Equal(t, "_", e.Server)
}

func TestParseErrorLineRejectsOtherLines(t *testing.T) {
	for _, line := range []string{
		"    at continuation of a previous message",
		"2026/09/07 15:32:29 [verbose] 1#1: x",
		"2026/13/07 15:32:29 [error] 1#1: x",
		`1.2.3.4 - - [07/Sep/2026:15:32:29 +0000] "GET / HTTP/1.1" 200 5 "-" "-"`,
	} {
		_, ok := ParseErrorLine(line)
		assert.False(t, ok, line)
	}
}

func TestErrorLevelOfTakesTheNginxNames(t *testing.T) {
	for input, want := range map[string]string{"warning": "warn", "ERROR": "error", "err": "error", "crit": "crit"} {
		got, ok := ErrorLevelOf(input)
		assert.True(t, ok, input)
		assert.Equal(t, want, got, input)
	}
	_, ok := ErrorLevelOf("fatal")
	assert.False(t, ok)
}
