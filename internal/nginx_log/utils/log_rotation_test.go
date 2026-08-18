package utils

import "testing"

func TestMainLogPathFromFile(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"/var/log/nginx/access.log", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.1", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.2.gz", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.20231201", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.2023-12-01", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.2023.12.01", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.1.log", "/var/log/nginx/access.log"},
		{"/var/log/nginx/error.log.14.bz2", "/var/log/nginx/error.log"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			if got := MainLogPathFromFile(tc.input); got != tc.expected {
				t.Errorf("MainLogPathFromFile(%s) = %s, want %s", tc.input, got, tc.expected)
			}
		})
	}
}

// TestMainLogPathFromFileDateExtRotation covers the file names logrotate writes
// when the `dateext` option is enabled, which is the default on Debian and
// Ubuntu. These have to collapse onto the same log group as the live log.
func TestMainLogPathFromFileDateExtRotation(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"/var/log/nginx/access.log-20231201", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log-20231201.gz", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log-2023-12-01", "/var/log/nginx/access.log"},
		{"/var/log/nginx/error.log-20231201.bz2", "/var/log/nginx/error.log"},
		{"/var/log/nginx/example.com.access.log-20231201", "/var/log/nginx/example.com.access.log"},
		// A name without a dotted suffix is not a rotated log: a site whose log
		// is literally called "site-20231201" must stay its own group.
		{"/var/log/nginx/site-20231201", "/var/log/nginx/site-20231201"},
		// A dash followed by something that is not a full date is left alone.
		{"/var/log/nginx/access.log-2023", "/var/log/nginx/access.log-2023"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			if got := MainLogPathFromFile(tc.input); got != tc.expected {
				t.Errorf("MainLogPathFromFile(%s) = %s, want %s", tc.input, got, tc.expected)
			}
		})
	}
}

func TestIsDatePattern(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"20231201", true},     // YYYYMMDD
		{"2023-12-01", true},   // YYYY-MM-DD
		{"2023.12.01", true},   // YYYY.MM.DD
		{"231201", true},       // YYMMDD
		{"access", false},      // Not a date
		{"123", false},         // Too short
		{"12345678901", false}, // Too long
		{"2023-13-01", true},   // Would match pattern (validation not checked)
		{"log", false},         // Text
		{"1", false},           // Single digit
	}

	for _, tc := range testCases {
		result := isDatePattern(tc.input)
		if result != tc.expected {
			t.Errorf("isDatePattern(%s) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}
