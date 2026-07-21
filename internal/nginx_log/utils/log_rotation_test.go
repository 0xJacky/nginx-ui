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
