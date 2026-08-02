package demo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// Synthetic access log.
//
// This is the one part of the demo that cannot be a provider. Everything the
// log dashboard shows is aggregated by the real parser, indexer and searcher
// from a file on disk, and there is no seam between "file" and "indexed
// document" — so the honest way to populate it is to write a real log and let
// the real pipeline consume it.
//
// The clients are drawn from the RFC 5737 documentation ranges, which is what
// geoService will fabricate a location for. That pairing is deliberate: the
// fake geo provider answers for exactly the addresses this file invents, and
// declines everything else.

const (
	// logLineCount is a compromise. Enough for the charts to have shape, few
	// enough that indexing them does not stretch the cold start; the bleve
	// index runs about 0.9 KB per line.
	logLineCount = 20000
	// logWindow is how far back the oldest entry sits. The dashboard's default
	// range is the last 24 hours, so the window must cover it.
	logWindow = 24 * time.Hour
)

// documentationClients enumerates the client addresses used in the log. A fixed
// pool rather than a random draw per line, so the "top clients" panel shows
// repeat visitors like a real site would.
func documentationClients() []string {
	clients := make([]string, 0, 180)
	for i := 1; i <= 60; i++ {
		clients = append(clients,
			fmt.Sprintf("203.0.113.%d", i),
			fmt.Sprintf("198.51.100.%d", i),
			fmt.Sprintf("192.0.2.%d", i),
		)
	}
	return clients
}

// statusFor picks a plausible status code for a request.
func statusFor(path string, s uint64) int {
	if probePaths[path] {
		// Scanner traffic: mostly refused, occasionally missing.
		if s%3 == 0 {
			return 403
		}
		return 404
	}

	switch {
	case s%97 == 0:
		return 500
	case s%53 == 0:
		return 502
	case s%29 == 0:
		return 304
	case s%23 == 0:
		return 404
	case writePaths[path] && s%7 == 0:
		return 201
	default:
		return 200
	}
}

func bytesFor(path string, status int, s uint64) int {
	if status == 304 {
		return 0
	}
	switch {
	case status >= 400:
		return rangeInt(s, 150, 600)
	case len(path) > 8 && path[:8] == "/assets/":
		return rangeInt(s, 40_000, 320_000)
	default:
		return rangeInt(s, 500, 9_000)
	}
}

// WriteAccessLogFixture generates the synthetic access log at path.
//
// Timestamps are anchored to an hour-quantised `now` rather than the exact boot
// time, so a container that restarts within the same hour produces a
// byte-identical file and the dashboard does not visibly jitter.
func WriteAccessLogFixture(path string, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1<<20)
	defer writer.Flush()

	anchor := now.Truncate(time.Hour)
	start := anchor.Add(-logWindow)
	step := logWindow / time.Duration(logLineCount)

	clients := documentationClients()

	for i := range logLineCount {
		s := seed("accesslog", fmt.Sprintf("%d", i))
		at := start.Add(time.Duration(i) * step)

		client := pick(clients, s)
		path := pick(requestPaths, s>>8)
		method := "GET"
		if writePaths[path] && s%5 == 0 {
			method = "POST"
		}
		status := statusFor(path, s>>16)

		// Combined format, matching log_format main in resources/demo/nginx.conf.
		fmt.Fprintf(writer,
			"%s - - [%s] \"%s %s HTTP/1.1\" %d %d \"%s\" \"%s\" \"-\"\n",
			client,
			at.Format("02/Jan/2006:15:04:05 -0700"),
			method, path, status, bytesFor(path, status, s>>24),
			pick(referers, s>>32),
			pick(userAgents, s>>40),
		)
	}

	return nil
}

// errorLogFixture writes a handful of nginx error lines so the error log view
// is not blank. Deliberately small: it exists to prove the viewer works, and
// a demo that looks like it is failing constantly is worse than a quiet one.
func writeErrorLogFixture(path string, now time.Time) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	anchor := now.Truncate(time.Hour)
	entries := []struct {
		offset time.Duration
		level  string
		text   string
	}{
		{-22 * time.Hour, "warn", "conflicting server name \"localhost\" on 0.0.0.0:8080, ignored"},
		{-18 * time.Hour, "error", "open() \"/var/www/html/.env\" failed (2: No such file or directory), client: 203.0.113.17, server: ojbk.me, request: \"GET /.env HTTP/1.1\""},
		{-11 * time.Hour, "error", "upstream timed out (110: Connection timed out) while reading response header from upstream, client: 198.51.100.42, server: langgood.com, upstream: \"http://127.0.0.1:9005/\""},
		{-6 * time.Hour, "warn", "an upstream response is buffered to a temporary file /tmp/nginx-proxy/0000000001 while reading upstream"},
		{-2 * time.Hour, "error", "connect() failed (111: Connection refused) while connecting to upstream, client: 192.0.2.9, server: langgood.com, upstream: \"http://127.0.0.1:9005/\""},
	}

	for _, e := range entries {
		at := anchor.Add(e.offset)
		fmt.Fprintf(file, "%s [%s] 1#1: *%d %s\n",
			at.Format("2006/01/02 15:04:05"), e.level,
			seed("errorlog", e.text)%9000+1000, e.text)
	}

	return nil
}

// installLogFixtures writes both logs, if they are not already populated.
//
// Refuses to overwrite a log that already has content: on a self-hosted
// instance that would destroy the operator's real access log, and the guard
// costs nothing.
func installLogFixtures() {
	now := time.Now()

	for _, fixture := range []struct {
		path  string
		write func(string, time.Time) error
	}{
		{"/var/log/nginx/access.log", WriteAccessLogFixture},
		{"/var/log/nginx/error.log", writeErrorLogFixture},
	} {
		if info, err := os.Stat(fixture.path); err == nil && info.Size() > 0 {
			logger.Infof("demo: %s already has content, leaving it alone", fixture.path)
			continue
		}

		if err := fixture.write(fixture.path, now); err != nil {
			logger.Errorf("demo: failed to write %s: %v", fixture.path, err)
			continue
		}
		logger.Infof("demo: wrote synthetic %s", fixture.path)
	}
}
