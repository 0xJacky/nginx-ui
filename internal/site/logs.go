package site

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// LogEntry represents a log file associated with a site.
type LogEntry struct {
	Type      string `json:"type"`      // "access" or "error"
	Path      string `json:"path"`      // absolute path to the log file
	Inherited bool   `json:"inherited"` // true when the site declares no directive and inherits the nginx default log
	Valid     bool   `json:"valid"`     // true when the path passes the log directory whitelist validation
}

// GetLogs resolves the access/error log paths of a site by scanning its
// configuration file. When the site declares no log directive of a given
// type, the nginx default log path is returned with Inherited set to true,
// since nginx falls back to the http-level configuration.
func GetLogs(name string) ([]LogEntry, error) {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}

	directives := utils.ScanLogDirectives(nginx.GetPrefix(), content)

	return buildLogEntries(directives, nginx.GetAccessLogPath(), nginx.GetErrorLogPath(), utils.IsValidLogPath), nil
}

// buildLogEntries deduplicates scanned log directives and falls back to the
// default access/error log paths for missing directive types.
func buildLogEntries(directives []utils.LogDirective, defaultAccessLog, defaultErrorLog string, isValid func(string) bool) []LogEntry {
	entries := make([]LogEntry, 0, len(directives))
	seen := make(map[string]bool)
	hasAccess := false
	hasError := false

	for _, directive := range directives {
		key := directive.Type + "|" + directive.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		switch directive.Type {
		case "access":
			hasAccess = true
		case "error":
			hasError = true
		}

		entries = append(entries, LogEntry{
			Type:  directive.Type,
			Path:  directive.Path,
			Valid: isValid(directive.Path),
		})
	}

	if !hasAccess && defaultAccessLog != "" {
		entries = append(entries, LogEntry{
			Type:      "access",
			Path:      defaultAccessLog,
			Inherited: true,
			Valid:     isValid(defaultAccessLog),
		})
	}

	if !hasError && defaultErrorLog != "" {
		entries = append(entries, LogEntry{
			Type:      "error",
			Path:      defaultErrorLog,
			Inherited: true,
			Valid:     isValid(defaultErrorLog),
		})
	}

	return entries
}
