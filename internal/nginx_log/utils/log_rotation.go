package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Rotation and date patterns used by MainLogPathFromFile. Compiled once at
// package level: this function is called in the per-line indexing hot path,
// and compiling regexes per call dominated the parse cost.
var (
	dotDateRotationRe  = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
	numberedRotationRe = regexp.MustCompile(`^(.+)\.(\d{1,3})$`)
	middleNumberedRe   = regexp.MustCompile(`^(.+)\.(\d{1,3})\.log$`)
	multiPartDateRe    = regexp.MustCompile(`^2\d{3}\.\d{2}\.\d{2}$`)
	fullDatePatternRes = []*regexp.Regexp{
		regexp.MustCompile(`^\d{8}$`),             // YYYYMMDD
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), // YYYY-MM-DD
		regexp.MustCompile(`^\d{6}$`),             // YYMMDD
	}
)

// MainLogPathFromFile extracts the main (base) log path from a file path,
// collapsing rotated and compressed variants (access.log.1, access.log.2.gz,
// access.log.20231201, ...) onto their log group base. This is the single
// canonical implementation: the value is persisted as MainLogPath in the
// index metadata and used for log group queries, so all grouping logic must
// agree with it.
func MainLogPathFromFile(filePath string) string {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)

	// Remove compression extensions (.gz, .bz2, .xz, .lz4)
	for _, ext := range []string{".gz", ".bz2", ".xz", ".lz4"} {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Check if it's a dot-separated date rotation FIRST (access.log.YYYYMMDD or access.log.YYYY.MM.DD)
	// This must come before numbered rotation check to avoid false positives
	parts := strings.Split(filename, ".")
	if len(parts) >= 3 {
		// First check for multi-part date patterns like YYYY.MM.DD (need at least 4 parts total)
		if len(parts) >= 4 {
			// Try to match the last 3 parts as a date
			lastThreeParts := strings.Join(parts[len(parts)-3:], ".")
			// Check if this looks like YYYY.MM.DD pattern
			if dotDateRotationRe.MatchString(lastThreeParts) {
				// Remove the date parts (last 3 parts)
				basenameParts := parts[:len(parts)-3]
				baseFilename := strings.Join(basenameParts, ".")
				return filepath.Join(dir, baseFilename)
			}
		}

		// Then check for single-part date patterns in the last part
		lastPart := parts[len(parts)-1]
		if isFullDatePattern(lastPart) { // Only match full date patterns, not partial ones
			// Remove the date part
			basenameParts := parts[:len(parts)-1]
			baseFilename := strings.Join(basenameParts, ".")
			return filepath.Join(dir, baseFilename)
		}
	}

	// Handle numbered rotation (access.log.1, access.log.2, etc.)
	// This comes AFTER date pattern checks to avoid matching date components as rotation numbers
	if match := numberedRotationRe.FindStringSubmatch(filename); len(match) > 1 {
		baseFilename := match[1]
		return filepath.Join(dir, baseFilename)
	}

	// Handle middle-numbered rotation (access.1.log, access.2.log)
	if match := middleNumberedRe.FindStringSubmatch(filename); len(match) > 1 {
		baseName := match[1]
		return filepath.Join(dir, baseName+".log")
	}

	// Handle date-based rotation (access.20231201, access.2023-12-01, etc.)
	if isDatePattern(filename) {
		// This is a date-based rotation, return the parent directory
		// as we can't determine the exact base name
		return filepath.Join(dir, "access.log") // Default assumption
	}

	// If no rotation pattern is found, return the original path
	return filePath
}

// isDatePattern checks if a string looks like a date pattern (including multi-part)
func isDatePattern(s string) bool {
	// Check for full date patterns first
	if isFullDatePattern(s) {
		return true
	}

	// Check for multi-part date patterns like YYYY.MM.DD
	return multiPartDateRe.MatchString(s)
}

// isFullDatePattern checks if a string is a complete date pattern (not partial)
func isFullDatePattern(s string) bool {
	for _, re := range fullDatePatternRes {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
