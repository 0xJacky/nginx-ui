package sink

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
)

// tailer follows a set of files like `tail -F`: a file seen at start is read
// from its end, a file that appears later or replaces a rotated one from its
// start, and a truncated one from its start again.
type tailer struct {
	cfg    Config
	parser *parser.Parser
	files  map[string]*followed
	now    func() time.Time
	// buf is reused by every read; lines are copied out of it.
	buf []byte
}

// followed is one access log.
type followed struct {
	path string
	// file is open while the path names a readable regular file.
	file *os.File
	// offset is the read position in file.
	offset int64
	// fromStart reads the next file opened at path from its first byte:
	// the path was missing before, or the previous file was rotated away.
	fromStart bool
	// partial holds a line whose line break has not been written yet.
	partial []byte
	// skipping drops the rest of a line that exceeded the length limit.
	skipping bool
}

func newTailer(cfg Config) *tailer {
	pc := parser.DefaultParserConfig()
	pc.EnableGeoIP = false
	pc.EnableUA = false
	pc.StrictMode = true
	pc.MaxLineLength = cfg.MaxLineLength
	return &tailer{
		cfg:    cfg,
		parser: parser.NewParser(pc, nil, nil),
		files:  map[string]*followed{},
		now:    time.Now,
	}
}

// refresh aligns the followed files with the paths of source. A path added
// later starts at the end of its file like one seen at start, only a file
// that appears where nothing was is read from its start.
func (t *tailer) refresh(source Source) {
	var paths []string
	if source.Paths != nil {
		paths = source.Paths()
	}

	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		if path == "" || wanted[path] || strings.HasSuffix(strings.ToLower(path), ".gz") {
			continue
		}
		if source.Allowed != nil && !source.Allowed(path) {
			continue
		}
		wanted[path] = true
		if _, ok := t.files[path]; ok {
			continue
		}
		f := &followed{path: path}
		t.files[path] = f
		t.open(f)
	}

	for path, f := range t.files {
		if !wanted[path] {
			f.closeFile()
			delete(t.files, path)
		}
	}
}

// open opens the file at the path of f. It returns false when there is none.
func (t *tailer) open(f *followed) bool {
	file, err := openLog(f.path)
	if err != nil {
		f.fromStart = true
		return false
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		// A special file such as /dev/stdout is not a log to follow.
		_ = file.Close()
		f.fromStart = true
		return false
	}

	f.file = file
	f.partial, f.skipping = nil, false
	if f.fromStart {
		f.offset = 0
	} else {
		f.offset = info.Size()
	}
	f.fromStart = false
	return true
}

// poll reads what was appended to every followed file since the last poll.
func (t *tailer) poll(ctx context.Context) []Entry {
	paths := make([]string, 0, len(t.files))
	for path := range t.files {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	var entries []Entry
	for _, path := range paths {
		if ctx.Err() != nil {
			return entries
		}
		entries = t.read(t.files[path], entries)
	}
	return entries
}

// read appends the new lines of one file to entries.
func (t *tailer) read(f *followed, entries []Entry) []Entry {
	if f.file == nil && !t.open(f) {
		return entries
	}

	info, err := f.file.Stat()
	if err != nil {
		f.closeFile()
		return entries
	}
	if info.Size() < f.offset {
		// Truncated in place (copytruncate): the new content starts at 0.
		f.offset = 0
		f.partial, f.skipping = nil, false
	}

	atEOF := true
	if remaining := info.Size() - f.offset; remaining > 0 {
		limit := min(remaining, t.cfg.MaxReadPerPoll)
		if int64(cap(t.buf)) < limit {
			t.buf = make([]byte, limit)
		}
		buf := t.buf[:limit]
		n, readErr := f.file.ReadAt(buf, f.offset)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			f.closeFile()
			return entries
		}
		f.offset += int64(n)
		atEOF = n == int(remaining)
		entries = t.split(f, buf[:n], entries)
	}

	if atEOF && t.rotated(f, info) {
		// The path names another file now. The old one is read to its end,
		// a line nginx left without a break ends with it, and the new file
		// is read from its start.
		if len(f.partial) > 0 && !f.skipping {
			entries = append(entries, t.entry(f.path, string(f.partial)))
		}
		f.closeFile()
		f.fromStart = true
		if t.open(f) {
			entries = t.read(f, entries)
		}
	}
	return entries
}

// rotated reports whether the path of f names another file than the open one.
func (t *tailer) rotated(f *followed, open os.FileInfo) bool {
	current, err := os.Stat(f.path)
	if err != nil {
		// Moved away and not recreated yet: keep reading the old file until
		// a new one appears.
		return false
	}
	return !os.SameFile(open, current)
}

// split cuts data into lines, keeping an unfinished one for the next poll.
func (t *tailer) split(f *followed, data []byte, entries []Entry) []Entry {
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			if !f.skipping {
				f.partial = append(f.partial, data...)
				if len(f.partial) > t.cfg.MaxLineLength {
					f.partial, f.skipping = nil, true
				}
			}
			return entries
		}

		chunk := data[:i]
		data = data[i+1:]
		if f.skipping {
			f.skipping = false
			continue
		}
		var line string
		if len(f.partial) > 0 {
			line = string(append(f.partial, chunk...))
			f.partial = nil
		} else {
			line = string(chunk)
		}
		line = strings.TrimSuffix(line, "\r")
		if line == "" || len(line) > t.cfg.MaxLineLength {
			continue
		}
		entries = append(entries, t.entry(f.path, line))
	}
	return entries
}

// entry parses one line. A line that is no combined format line is kept raw.
func (t *tailer) entry(path, line string) Entry {
	parsed, err := t.parser.ParseLine(line)
	if err != nil || parsed == nil || parsed.Status == 0 || parsed.Timestamp <= 0 {
		return Entry{LogPath: path, Time: t.now().UTC(), Raw: line}
	}

	e := Entry{
		LogPath:       path,
		Time:          time.Unix(parsed.Timestamp, 0).UTC(),
		Parsed:        true,
		RemoteAddr:    parsed.IP,
		Method:        parsed.Method,
		URI:           parsed.Path,
		Protocol:      parsed.Protocol,
		Status:        parsed.Status,
		BodyBytesSent: parsed.BytesSent,
		Referer:       dash(parsed.Referer),
		UserAgent:     dash(parsed.UserAgent),
		RequestTime:   parsed.RequestTime,
		Raw:           line,
	}
	if parsed.UpstreamTime != nil {
		e.UpstreamResponseTime = *parsed.UpstreamTime
	}
	return e
}

// dash maps the "-" nginx writes for an empty value to "".
func dash(value string) string {
	if value == "-" {
		return ""
	}
	return value
}

func (t *tailer) close() {
	for _, f := range t.files {
		f.closeFile()
	}
}

func (f *followed) closeFile() {
	if f.file != nil {
		_ = f.file.Close()
		f.file = nil
	}
	f.partial, f.skipping = nil, false
}
