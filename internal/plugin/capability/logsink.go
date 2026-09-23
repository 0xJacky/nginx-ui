package capability

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/sink"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// LogSinkHost is the part of the plugin manager the log.sink capability
// needs.
type LogSinkHost interface {
	// SetLogFeed connects the log.sink plugins to the access log feed.
	SetLogFeed(feed plugin.LogFeed)
}

// RegisterLogSink streams the access log lines of the host to every enabled
// log.sink plugin. The feeder of the hub only runs while one of them is up.
func RegisterLogSink(h LogSinkHost) {
	h.SetLogFeed(NewLogFeed(sink.Default()))
}

// NewLogFeed exposes the lines of a hub as the log feed of the plugin
// manager. Every batch is converted once and shared by all plugins.
func NewLogFeed(hub *sink.Hub) plugin.LogFeed {
	return logFeed{hub: hub}
}

type logFeed struct {
	hub *sink.Hub
}

func (f logFeed) Subscribe(deliver func(entries []protocol.LogSinkPushParams)) (unsubscribe func()) {
	return f.hub.Subscribe(sink.SubscriberFunc(func(entries []sink.Entry) {
		deliver(LogEntries(entries))
	}))
}

// LogEntries converts the lines of the hub into log.push messages (spec
// LOGSINK-6).
func LogEntries(entries []sink.Entry) []protocol.LogSinkPushParams {
	out := make([]protocol.LogSinkPushParams, len(entries))
	for i, e := range entries {
		entry := protocol.LogEntry{
			Timestamp: e.Time.UTC().Format(time.RFC3339),
			Raw:       e.Raw,
			Format:    protocol.LogFormatRaw,
		}
		if e.Parsed {
			entry.Format = protocol.LogFormatCombined
			entry.RemoteAddr = e.RemoteAddr
			entry.RequestMethod = e.Method
			entry.RequestURI = e.URI
			entry.Protocol = e.Protocol
			entry.Status = e.Status
			entry.BodyBytesSent = protocol.ByteSize(e.BodyBytesSent)
			entry.Referer = e.Referer
			entry.UserAgent = e.UserAgent
			entry.UpstreamAddr = e.UpstreamAddr
			entry.RequestTime = e.RequestTime
			entry.UpstreamResponseTime = e.UpstreamResponseTime
			entry.Host = e.Host
		}
		if e.Time.IsZero() {
			entry.Timestamp = ""
		}
		out[i] = protocol.LogSinkPushParams{LogPath: e.LogPath, Entry: entry}
	}
	return out
}
