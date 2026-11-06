// Ported from the official log analytics plugin (nginxui/plugin-log-analytics).

package parser

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// maxErrorLineLength is the longest error log line that is parsed, like the
// default limit of the access log parser.
const maxErrorLineLength = 16 * 1024

// ErrorLevels are the severity levels of the nginx error log, from the lowest.
var ErrorLevels = []string{"debug", "info", "notice", "warn", "error", "crit", "alert", "emerg"}

// ErrorLogEntry is one parsed line of the nginx error log.
type ErrorLogEntry struct {
	Timestamp int64
	Level     string
	PID       int64
	// Connection is the connection number, zero when the line names none.
	Connection int64
	Message    string
	Client     string
	Server     string
	Method     string
	Path       string
	Request    string
	Upstream   string
	Host       string
	Referrer   string
}

// errorContextKeys is the context nginx appends to a message, in the order it
// writes it. Quoted values escape their own quotes as \x22.
var errorContextKeys = []struct {
	key    string
	quoted bool
}{
	{", client: ", false},
	{", server: ", false},
	{", request: ", true},
	{", upstream: ", true},
	{", host: ", true},
	{", referrer: ", true},
}

// ErrorLevelOf returns an error log level as the index keeps it, or false for
// an unknown one. The names warning, err, critical and emergency are taken for
// the ones nginx writes.
func ErrorLevelOf(value string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(value))
	switch name {
	case "warning":
		name = "warn"
	case "err":
		name = "error"
	case "critical":
		name = "crit"
	case "emergency":
		name = "emerg"
	}
	if slices.Contains(ErrorLevels, name) {
		return name, true
	}
	return "", false
}

// ParseErrorLine parses one error log line of the form
// `2026/09/07 15:32:29 [error] 12#12: *34 message, client: 1.2.3.4, ...`.
// The time is read in the local zone of the server. The second result is
// false when the line does not start like an error log entry.
func ParseErrorLine(line string) (*ErrorLogEntry, bool) {
	if len(line) < 25 || len(line) > maxErrorLineLength {
		return nil, false
	}
	ts, err := time.ParseInLocation("2006/01/02 15:04:05", line[:19], time.Local)
	if err != nil {
		return nil, false
	}
	rest, ok := strings.CutPrefix(line[19:], " [")
	if !ok {
		return nil, false
	}
	end := strings.IndexByte(rest, ']')
	if end < 0 || !slices.Contains(ErrorLevels, rest[:end]) {
		return nil, false
	}
	e := &ErrorLogEntry{Timestamp: ts.Unix(), Level: rest[:end]}
	rest = strings.TrimLeft(rest[end+1:], " ")

	// "12#34: " names the process and the thread
	if colon := strings.Index(rest, ": "); colon >= 0 {
		if pid, tid, found := strings.Cut(rest[:colon], "#"); found && isDigits(tid) {
			if n, err := strconv.ParseInt(pid, 10, 64); err == nil {
				e.PID = n
				rest = rest[colon+2:]
			}
		}
	}
	// "*56 " names the connection
	if after, found := strings.CutPrefix(rest, "*"); found {
		digits := 0
		for digits < len(after) && after[digits] >= '0' && after[digits] <= '9' {
			digits++
		}
		if digits > 0 && digits < len(after) && after[digits] == ' ' {
			e.Connection, _ = strconv.ParseInt(after[:digits], 10, 64)
			rest = after[digits+1:]
		}
	}

	// The message ends where the first context key starts
	first := len(rest)
	for _, c := range errorContextKeys {
		if i := strings.Index(rest, c.key); i >= 0 && i < first {
			first = i
		}
	}
	e.Message = rest[:first]
	tail := rest[first:]
	for _, c := range errorContextKeys {
		after, found := strings.CutPrefix(tail, c.key)
		if !found {
			continue
		}
		var value, next string
		if c.quoted {
			inner, isQuoted := strings.CutPrefix(after, `"`)
			closing := strings.IndexByte(inner, '"')
			if !isQuoted || closing < 0 {
				break
			}
			value, next = inner[:closing], inner[closing+1:]
		} else {
			stop := strings.Index(after, ", ")
			if stop < 0 {
				stop = len(after)
			}
			value, next = after[:stop], after[stop:]
		}
		switch c.key {
		case ", client: ":
			e.Client = value
		case ", server: ":
			e.Server = value
		case ", request: ":
			e.Request = value
		case ", upstream: ":
			e.Upstream = value
		case ", host: ":
			e.Host = value
		default:
			e.Referrer = value
		}
		tail = strings.TrimLeft(next, " ")
		if tail != "" && !strings.HasPrefix(tail, ", ") {
			break
		}
	}
	if fields := strings.Fields(e.Request); len(fields) >= 2 && validRequestMethods[fields[0]] {
		e.Method, e.Path = fields[0], fields[1]
	}
	return e, true
}

// validRequestMethods are the HTTP and WebDAV methods a request in an error
// log line may name.
var validRequestMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "PATCH": true, "CONNECT": true, "TRACE": true,
	"PROPFIND": true, "PROPPATCH": true, "MKCOL": true,
	"COPY": true, "MOVE": true, "LOCK": true, "UNLOCK": true,
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
