package nginx_log

import (
	"testing"
	"time"
)

func TestLogParser_ParseLine(t *testing.T) {
	// Create mock user agent parser
	mockUA := NewMockUserAgentParser()

	// Add test responses
	mockUA.AddResponse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", UserAgentInfo{
		Browser:    "Chrome",
		BrowserVer: "91.0",
		OS:         "Windows 10",
		OSVersion:  "10.0",
		DeviceType: "Desktop",
	})

	parser := NewLogParser(mockUA)

	testCases := []struct {
		name     string
		logLine  string
		expected *AccessLogEntry
		wantErr  bool
	}{
		{
			name:    "Combined log format",
			logLine: `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /api/test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" 0.123 0.050`,
			expected: &AccessLogEntry{
				IP:           "192.168.1.1",
				Timestamp:    time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC),
				Method:       "GET",
				Path:         "/api/test",
				Protocol:     "HTTP/1.1",
				Status:       200,
				BytesSent:    1024,
				Referer:      "https://example.com",
				UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				RequestTime:  0.123,
				UpstreamTime: 0.050,
				Browser:      "Chrome",
				BrowserVer:   "91.0",
				OS:           "Windows 10",
				OSVersion:    "10.0",
				DeviceType:   "Desktop",
			},
			wantErr: false,
		},
		{
			name:    "Common log format (no user agent)",
			logLine: `10.0.0.1 - - [01/Jan/2023:12:00:00 +0000] "POST /submit HTTP/1.1" 201 512`,
			expected: &AccessLogEntry{
				IP:        "10.0.0.1",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Method:    "POST",
				Path:      "/submit",
				Protocol:  "HTTP/1.1",
				Status:    201,
				BytesSent: 512,
			},
			wantErr: false,
		},
		{
			name:    "Invalid log line",
			logLine: "invalid log line format",
			wantErr: true,
		},
		{
			name:    "Empty log line",
			logLine: "",
			wantErr: true,
		},
		{
			name:    "Log line with missing fields",
			logLine: `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test"`,
			wantErr: true,
		},
		{
			name:    "Log with special characters in path",
			logLine: `127.0.0.1 - - [01/Jan/2023:00:00:00 +0000] "GET /path%20with%20spaces?param=value HTTP/1.1" 200 0 "-" "-"`,
			expected: &AccessLogEntry{
				IP:        "127.0.0.1",
				Timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				Method:    "GET",
				Path:      "/path%20with%20spaces?param=value",
				Protocol:  "HTTP/1.1",
				Status:    200,
				BytesSent: 0,
				Referer:   "-",
				UserAgent: "-",
			},
			wantErr: false,
		},
		{
			name:    "Log with IPv6 address",
			logLine: `2001:db8::1 - - [01/Jan/2023:00:00:00 +0000] "GET /ipv6 HTTP/1.1" 200 100 "-" "-"`,
			expected: &AccessLogEntry{
				IP:        "2001:db8::1",
				Timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				Method:    "GET",
				Path:      "/ipv6",
				Protocol:  "HTTP/1.1",
				Status:    200,
				BytesSent: 100,
				Referer:   "-",
				UserAgent: "-",
			},
			wantErr: false,
		},
		{
			name:    "Log with high status code",
			logLine: `192.168.1.1 - - [01/Jan/2023:00:00:00 +0000] "GET /error HTTP/1.1" 500 0 "-" "-"`,
			expected: &AccessLogEntry{
				IP:        "192.168.1.1",
				Timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				Method:    "GET",
				Path:      "/error",
				Protocol:  "HTTP/1.1",
				Status:    500,
				BytesSent: 0,
				Referer:   "-",
				UserAgent: "-",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.ParseLine(tc.logLine)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			// Check each field
			if result.IP != tc.expected.IP {
				t.Errorf("IP mismatch. Expected: %s, Got: %s", tc.expected.IP, result.IP)
			}
			if !result.Timestamp.Equal(tc.expected.Timestamp) {
				t.Errorf("Timestamp mismatch. Expected: %v, Got: %v", tc.expected.Timestamp, result.Timestamp)
			}
			if result.Method != tc.expected.Method {
				t.Errorf("Method mismatch. Expected: %s, Got: %s", tc.expected.Method, result.Method)
			}
			if result.Path != tc.expected.Path {
				t.Errorf("Path mismatch. Expected: %s, Got: %s", tc.expected.Path, result.Path)
			}
			if result.Protocol != tc.expected.Protocol {
				t.Errorf("Protocol mismatch. Expected: %s, Got: %s", tc.expected.Protocol, result.Protocol)
			}
			if result.Status != tc.expected.Status {
				t.Errorf("Status mismatch. Expected: %d, Got: %d", tc.expected.Status, result.Status)
			}
			if result.BytesSent != tc.expected.BytesSent {
				t.Errorf("BytesSent mismatch. Expected: %d, Got: %d", tc.expected.BytesSent, result.BytesSent)
			}
			if result.Referer != tc.expected.Referer {
				t.Errorf("Referer mismatch. Expected: %s, Got: %s", tc.expected.Referer, result.Referer)
			}
			if result.UserAgent != tc.expected.UserAgent {
				t.Errorf("UserAgent mismatch. Expected: %s, Got: %s", tc.expected.UserAgent, result.UserAgent)
			}
			if result.RequestTime != tc.expected.RequestTime {
				t.Errorf("RequestTime mismatch. Expected: %f, Got: %f", tc.expected.RequestTime, result.RequestTime)
			}
			if result.UpstreamTime != tc.expected.UpstreamTime {
				t.Errorf("UpstreamTime mismatch. Expected: %f, Got: %f", tc.expected.UpstreamTime, result.UpstreamTime)
			}
			if result.Browser != tc.expected.Browser {
				t.Errorf("Browser mismatch. Expected: %s, Got: %s", tc.expected.Browser, result.Browser)
			}
			if result.BrowserVer != tc.expected.BrowserVer {
				t.Errorf("BrowserVer mismatch. Expected: %s, Got: %s", tc.expected.BrowserVer, result.BrowserVer)
			}
			if result.OS != tc.expected.OS {
				t.Errorf("OS mismatch. Expected: %s, Got: %s", tc.expected.OS, result.OS)
			}
			if result.OSVersion != tc.expected.OSVersion {
				t.Errorf("OSVersion mismatch. Expected: %s, Got: %s", tc.expected.OSVersion, result.OSVersion)
			}
			if result.DeviceType != tc.expected.DeviceType {
				t.Errorf("DeviceType mismatch. Expected: %s, Got: %s", tc.expected.DeviceType, result.DeviceType)
			}
		})
	}
}

func TestLogParser_ParseTimestamp(t *testing.T) {
	parser := NewLogParser(NewMockUserAgentParser())

	testCases := []struct {
		name      string
		timestamp string
		expected  time.Time
		wantErr   bool
	}{
		{
			name:      "Valid timestamp with timezone",
			timestamp: "25/Dec/2023:10:00:00 +0000",
			expected:  time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "Valid timestamp with negative timezone",
			timestamp: "01/Jan/2023:12:00:00 -0500",
			expected:  time.Date(2023, 1, 1, 17, 0, 0, 0, time.UTC), // UTC equivalent
			wantErr:   false,
		},
		{
			name:      "Valid timestamp with positive timezone",
			timestamp: "15/Jun/2023:14:30:45 +0200",
			expected:  time.Date(2023, 6, 15, 12, 30, 45, 0, time.UTC), // UTC equivalent
			wantErr:   false,
		},
		{
			name:      "Invalid timestamp format",
			timestamp: "invalid timestamp",
			wantErr:   true,
		},
		{
			name:      "Empty timestamp",
			timestamp: "",
			wantErr:   true,
		},
		{
			name:      "Timestamp with invalid date",
			timestamp: "32/Dec/2023:10:00:00 +0000",
			wantErr:   true,
		},
		{
			name:      "Timestamp with invalid month",
			timestamp: "01/InvalidMonth/2023:10:00:00 +0000",
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.parseTimestamp(tc.timestamp)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !result.Equal(tc.expected) {
				t.Errorf("Timestamp mismatch. Expected: %v, Got: %v", tc.expected, result)
			}
		})
	}
}

func TestDetectLogFormat(t *testing.T) {
	testCases := []struct {
		name     string
		logLine  string
		expected LogFormat
	}{
		{
			name:     "Combined format",
			logLine:  `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0" 0.123`,
			expected: LogFormatCombined,
		},
		{
			name:     "Common format",
			logLine:  `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024`,
			expected: LogFormatCommon,
		},
		{
			name:     "Unknown format",
			logLine:  "invalid log line",
			expected: LogFormatUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectLogFormat(tc.logLine)
			if result != tc.expected {
				t.Errorf("Format mismatch. Expected: %d, Got: %d", tc.expected, result)
			}
		})
	}
}