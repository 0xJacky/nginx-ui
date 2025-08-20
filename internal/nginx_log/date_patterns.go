package nginx_log

import (
	"regexp"
	"strings"
)

// LogRotationDatePatterns defines common log rotation date patterns
var LogRotationDatePatterns = []string{
	`^\d{8}$`,                   // YYYYMMDD
	`^\d{4}-\d{2}-\d{2}$`,       // YYYY-MM-DD
	`^\d{4}\.\d{2}\.\d{2}$`,     // YYYY.MM.DD
	`^\d{4}_\d{2}_\d{2}$`,       // YYYY_MM_DD
	`^\d{10}$`,                  // YYYYMMDDHH
	`^\d{12}$`,                  // YYYYMMDDHHMI
	`^\d{4}-\d{2}-\d{2}_\d{2}$`, // YYYY-MM-DD_HH
}

// isDatePattern checks if a string matches any date pattern
func isDatePattern(s string) bool {
	for _, pattern := range LogRotationDatePatterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return true
		}
	}
	return false
}

// isCompressedDatePattern checks if a string is a compressed date pattern (e.g., "20231201.gz")
func isCompressedDatePattern(s string) bool {
	if !strings.HasSuffix(s, ".gz") {
		return false
	}
	
	datePart := strings.TrimSuffix(s, ".gz")
	return isDatePattern(datePart)
}

// isNumberPattern checks if a string is a numeric rotation pattern (e.g., "1", "2", "10")
func isNumberPattern(s string) bool {
	matched, _ := regexp.MatchString(`^\d+$`, s)
	return matched
}

// isCompressedNumberPattern checks if a string is a compressed numeric pattern (e.g., "1.gz", "2.gz")
func isCompressedNumberPattern(s string) bool {
	if !strings.HasSuffix(s, ".gz") {
		return false
	}
	
	numberPart := strings.TrimSuffix(s, ".gz")
	return isNumberPattern(numberPart)
}

// isLogrotateFile determines if a file is a logrotate-generated file
func isLogrotateFile(filename, baseLogName string) bool {
	// If filename equals baseLogName, it's the active log file
	if filename == baseLogName {
		return true
	}

	// Check if it starts with the base log name
	if !strings.HasPrefix(filename, baseLogName) {
		return false
	}

	// Extract the suffix after the base log name
	suffix := strings.TrimPrefix(filename, baseLogName)
	if !strings.HasPrefix(suffix, ".") {
		return false
	}
	suffix = strings.TrimPrefix(suffix, ".")

	// Check various rotation patterns
	return isNumberPattern(suffix) ||
		isCompressedNumberPattern(suffix) ||
		isDatePattern(suffix) ||
		isCompressedDatePattern(suffix)
}