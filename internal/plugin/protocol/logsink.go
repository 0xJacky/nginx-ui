package protocol

// LogSinkPushParams is one message of the log.push stream: one access log
// line. log.push is a client stream on the gRPC transport only and has no
// JSON-RPC form (spec WIRE-12); the JSON tags document the protobuf JSON
// mapping of the message.
type LogSinkPushParams struct {
	// LogPath is the absolute path of the access log the line was read from.
	LogPath string   `json:"log_path"`
	Entry   LogEntry `json:"entry"`
}

// LogEntry is one access log line. Fields the host could not extract are
// empty.
type LogEntry struct {
	// Timestamp is the time of the request in RFC 3339 form.
	Timestamp     string   `json:"timestamp,omitempty"`
	RemoteAddr    string   `json:"remote_addr,omitempty"`
	RequestMethod string   `json:"request_method,omitempty"`
	RequestURI    string   `json:"request_uri,omitempty"`
	Protocol      string   `json:"protocol,omitempty"`
	Status        int      `json:"status,omitempty"`
	BodyBytesSent ByteSize `json:"body_bytes_sent,omitempty"`
	Referer       string   `json:"referer,omitempty"`
	UserAgent     string   `json:"user_agent,omitempty"`
	UpstreamAddr  string   `json:"upstream_addr,omitempty"`
	// RequestTime is $request_time in seconds.
	RequestTime float64 `json:"request_time,omitempty"`
	// UpstreamResponseTime is $upstream_response_time in seconds.
	UpstreamResponseTime float64 `json:"upstream_response_time,omitempty"`
	Host                 string  `json:"host,omitempty"`
	// Raw is the line as nginx wrote it.
	Raw string `json:"raw,omitempty"`
	// Format is LogFormatCombined or LogFormatRaw.
	Format string `json:"format,omitempty"`
}

// LogSinkPushResult is the answer to one log.push stream.
type LogSinkPushResult struct {
	// Accepted counts the entries the plugin kept.
	Accepted int `json:"accepted"`
	// Rejected counts the entries the plugin received and discarded.
	Rejected int `json:"rejected"`
}

// Values of LogEntry.Format and of ManifestLogSink.Formats.
const (
	// LogFormatCombined is a line the host parsed as the nginx combined
	// format, optionally followed by $request_time and
	// $upstream_response_time.
	LogFormatCombined = "combined"
	// LogFormatRaw is any other line. Only Raw and Timestamp are set.
	LogFormatRaw = "raw"
)
