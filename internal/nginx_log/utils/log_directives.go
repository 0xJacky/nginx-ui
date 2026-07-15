package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// LogDirective represents a parsed access_log or error_log directive
// found in an nginx configuration file.
type LogDirective struct {
	Type string // "access" or "error"
	Path string // absolute log file path (relative paths resolved against the nginx prefix)
}

// logDirectiveRegex matches access_log or error_log directives and captures
// the directive name and its first parameter (the log target).
var logDirectiveRegex = regexp.MustCompile(`(?m)(access_log|error_log)\s+([^\s;]+)(?:\s+[^;]+)?;`)

// ScanLogDirectives extracts access_log/error_log directives from nginx
// configuration content. Commented directives and non-file targets
// (off, stderr, stdout, syslog:, memory:) are skipped. Relative paths are
// resolved against the given nginx prefix.
func ScanLogDirectives(prefix string, content []byte) []LogDirective {
	matches := logDirectiveRegex.FindAllSubmatchIndex(content, -1)
	directives := make([]LogDirective, 0, len(matches))

	for _, m := range matches {
		// m holds pair offsets: [full, full, group1, group1, group2, group2]
		if isCommentedAt(content, m[0]) {
			continue
		}

		directiveType := string(content[m[2]:m[3]])
		logPath := string(content[m[4]:m[5]])

		if !isFileTarget(logPath) {
			continue
		}

		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(prefix, logPath)
		}

		logType := "access"
		if directiveType == "error_log" {
			logType = "error"
		}

		directives = append(directives, LogDirective{
			Type: logType,
			Path: logPath,
		})
	}

	return directives
}

// isFileTarget reports whether a log directive parameter points to a regular
// file rather than a special target such as off, stderr, stdout, syslog or memory.
func isFileTarget(logPath string) bool {
	switch logPath {
	case "off", "stderr", "stdout":
		return false
	}
	return !strings.HasPrefix(logPath, "syslog:") && !strings.HasPrefix(logPath, "memory:")
}

// isCommentedAt checks whether the directive starting at offset is on a
// commented line (i.e. preceded by '#' after the start of its line).
func isCommentedAt(content []byte, offset int) bool {
	lineStart := offset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	for i := lineStart; i < offset; i++ {
		switch content[i] {
		case '#':
			return true
		case ' ', '\t':
			continue
		default:
			return false
		}
	}

	return false
}
