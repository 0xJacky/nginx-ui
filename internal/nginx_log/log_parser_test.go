package nginx_log

import (
	"testing"
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
				IP:          "192.168.1.1",
				Method:      "GET",
				Path:        "/api/test",
				Protocol:    "HTTP/1.1",
				Status:      200,
				BytesSent:   1024,
				Referer:     "https://example.com",
				UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				Browser:     "Chrome",
				BrowserVer:  "91.0",
				OS:          "Windows 10",
				OSVersion:   "10.0",
				DeviceType:  "Desktop",
				RequestTime: 0.123,
			},
			wantErr: false,
		},
		{
			name:    "Main log format",
			logLine: `10.0.0.1 - user [25/Dec/2023:10:00:00 +0000] "POST /login HTTP/1.1" 302 0 "-" "curl/7.68.0"`,
			expected: &AccessLogEntry{
				IP:         "10.0.0.1",
				Method:     "POST",
				Path:       "/login",
				Protocol:   "HTTP/1.1",
				Status:     302,
				BytesSent:  0,
				UserAgent:  "curl/7.68.0",
				Browser:    "Unknown", // curl doesn't match our browser patterns
				DeviceType: "Desktop",
			},
			wantErr: false,
		},
		{
			name:    "Empty line",
			logLine: "",
			wantErr: true,
		},
		{
			name:    "Invalid format",
			logLine: "this is not a valid log line",
			expected: &AccessLogEntry{
				Raw: "this is not a valid log line",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.ParseLine(tc.logLine)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			// Check key fields
			if tc.expected != nil {
				if result.IP != tc.expected.IP {
					t.Errorf("expected IP %q, got %q", tc.expected.IP, result.IP)
				}
				if result.Method != tc.expected.Method {
					t.Errorf("expected Method %q, got %q", tc.expected.Method, result.Method)
				}
				if result.Status != tc.expected.Status {
					t.Errorf("expected Status %d, got %d", tc.expected.Status, result.Status)
				}
				if result.Browser != tc.expected.Browser {
					t.Errorf("expected Browser %q, got %q", tc.expected.Browser, result.Browser)
				}
			}
		})
	}
}

func TestLogParser_ParseTimestamp(t *testing.T) {
	parser := &LogParser{}

	testCases := []struct {
		name      string
		timestamp string
		wantErr   bool
	}{
		{
			name:      "Standard nginx format",
			timestamp: "25/Dec/2023:10:00:00 +0000",
			wantErr:   false,
		},
		{
			name:      "ISO format",
			timestamp: "2023-12-25T10:00:00-07:00",
			wantErr:   false,
		},
		{
			name:      "Invalid format",
			timestamp: "invalid timestamp",
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.parseTimestamp(tc.timestamp)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result.IsZero() {
				t.Errorf("expected valid time but got zero time")
			}
		})
	}
}

func TestDetectLogFormat(t *testing.T) {
	testLines := []string{
		`192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0"`,
		`10.0.0.1 - user [25/Dec/2023:10:00:00 +0000] "POST /login HTTP/1.1" 302 0 "-" "curl/7.68.0"`,
		`192.168.1.2 - - [25/Dec/2023:10:01:00 +0000] "GET /api/data HTTP/1.1" 200 2048 "https://example.com" "Mozilla/5.0"`,
	}

	format := DetectLogFormat(testLines)

	if format == nil {
		t.Errorf("expected to detect a format but got nil")
		return
	}

	if format.Name != "main" && format.Name != "combined" {
		t.Errorf("expected to detect main or combined format, got %q", format.Name)
	}
}

func TestSimpleUserAgentParser_Parse(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name      string
		userAgent string
		expected  UserAgentInfo
	}{
		{
			name:      "Chrome on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expected: UserAgentInfo{
				Browser:    "Chrome",
				BrowserVer: "91.0",
				OS:         "Windows 10",
				DeviceType: "Desktop",
			},
		},
		{
			name:      "Mobile Safari",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			expected: UserAgentInfo{
				Browser:    "Safari",
				OS:         "iOS",
				DeviceType: "iPhone",
			},
		},
		{
			name:      "Bot",
			userAgent: "Googlebot/2.1 (+http://www.google.com/bot.html)",
			expected: UserAgentInfo{
				Browser:    "Unknown",
				OS:         "Unknown",
				DeviceType: "Bot",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.Browser != tc.expected.Browser {
				t.Errorf("expected Browser %q, got %q", tc.expected.Browser, result.Browser)
			}
			if result.DeviceType != tc.expected.DeviceType {
				t.Errorf("expected DeviceType %q, got %q", tc.expected.DeviceType, result.DeviceType)
			}
		})
	}
}

func BenchmarkLogParser_ParseLine(b *testing.B) {
	mockUA := NewMockUserAgentParser()
	parser := NewLogParser(mockUA)

	logLine := `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(logLine)
	}
}

func BenchmarkUserAgentParser_Parse(b *testing.B) {
	parser := NewSimpleUserAgentParser()
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.Parse(userAgent)
	}
}

func TestUserAgentVersionParsing(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name            string
		userAgent       string
		expectedBrowser string
		expectedVersion string
	}{
		{
			name:            "Chrome with correct version",
			userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expectedBrowser: "Chrome",
			expectedVersion: "91.0",
		},
		{
			name:            "Safari with correct version",
			userAgent:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
			expectedBrowser: "Safari",
			expectedVersion: "14.1",
		},
		{
			name:            "Firefox with correct version",
			userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
			expectedBrowser: "Firefox",
			expectedVersion: "89.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.Browser != tc.expectedBrowser {
				t.Errorf("Expected browser %q, got %q", tc.expectedBrowser, result.Browser)
			}

			if result.BrowserVer != tc.expectedVersion {
				t.Errorf("Expected version %q, got %q", tc.expectedVersion, result.BrowserVer)
			}
		})
	}
}

func TestOSVersionParsing(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name          string
		userAgent     string
		expectedOS    string
		expectedOSVer string
	}{
		{
			name:          "iOS with correct version and device",
			userAgent:     "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expectedOS:    "iOS",
			expectedOSVer: "14.6",
		},
		{
			name:          "Android with correct version",
			userAgent:     "Mozilla/5.0 (Linux; Android 11.0; SM-G991B) AppleWebKit/537.36",
			expectedOS:    "Android",
			expectedOSVer: "11.0",
		},
		{
			name:          "Windows 10",
			userAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			expectedOS:    "Windows 10",
			expectedOSVer: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.OS != tc.expectedOS {
				t.Errorf("Expected OS %q, got %q", tc.expectedOS, result.OS)
			}

			if result.OSVersion != tc.expectedOSVer {
				t.Errorf("Expected OS version %q, got %q", tc.expectedOSVer, result.OSVersion)
			}
		})
	}
}

func TestDeviceTypeParsing(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name               string
		userAgent          string
		expectedDeviceType string
	}{
		{
			name:               "iPhone device",
			userAgent:          "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expectedDeviceType: "iPhone",
		},
		{
			name:               "iPad device",
			userAgent:          "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expectedDeviceType: "iPad",
		},
		{
			name:               "Android Mobile",
			userAgent:          "Mozilla/5.0 (Linux; Android 11.0; SM-G991B) AppleWebKit/537.36 Mobile",
			expectedDeviceType: "Mobile",
		},
		{
			name:               "Android Tablet",
			userAgent:          "Mozilla/5.0 (Linux; Android 11.0; SM-T720) AppleWebKit/537.36",
			expectedDeviceType: "Tablet",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.DeviceType != tc.expectedDeviceType {
				t.Errorf("Expected device type %q, got %q", tc.expectedDeviceType, result.DeviceType)
			}
		})
	}
}

func TestAndroidOSParsing(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name      string
		userAgent string
		wantOS    string
		wantVer   string
	}{
		{
			name:      "Android 13",
			userAgent: "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
			wantVer:   "13.0",
		},
		{
			name:      "Android 12",
			userAgent: "Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
			wantVer:   "12.0",
		},
		{
			name:      "Android 11",
			userAgent: "Mozilla/5.0 (Linux; Android 11; SM-A515F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
			wantVer:   "11.0",
		},
		{
			name:      "Android 10",
			userAgent: "Mozilla/5.0 (Linux; Android 10; SM-G973F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
			wantVer:   "10.0",
		},
		{
			name:      "Android 9",
			userAgent: "Mozilla/5.0 (Linux; Android 9; SM-G960F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
			wantVer:   "9.0",
		},
		{
			name:      "Android with minor version",
			userAgent: "Mozilla/5.0 (Linux; Android 8.1; Nexus 5X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
			wantVer:   "8.1",
		},
		{
			name:      "Android vs Linux priority test",
			userAgent: "Mozilla/5.0 (Linux; Android 12; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android", // Should be Android, not Linux
			wantVer:   "12.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)
			if result.OS != tc.wantOS {
				t.Errorf("Expected OS %s, got %s for User-Agent: %s", tc.wantOS, result.OS, tc.userAgent)
			}
			if result.OSVersion != tc.wantVer {
				t.Errorf("Expected OS version %s, got %s for User-Agent: %s", tc.wantVer, result.OSVersion, tc.userAgent)
			}
		})
	}
}
func TestRealWorldAndroidUserAgents(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	// Real Android User-Agent strings from various devices
	testCases := []struct {
		name      string
		userAgent string
		wantOS    string
	}{
		{
			name:      "Samsung Galaxy S21",
			userAgent: "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
		},
		{
			name:      "Google Pixel 6",
			userAgent: "Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.45 Mobile Safari/537.36",
			wantOS:    "Android",
		},
		{
			name:      "OnePlus 9",
			userAgent: "Mozilla/5.0 (Linux; Android 11; LE2113) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
		},
		{
			name:      "Xiaomi Mi 11",
			userAgent: "Mozilla/5.0 (Linux; Android 11; M2011K2C) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			wantOS:    "Android",
		},
		{
			name:      "Android Tablet",
			userAgent: "Mozilla/5.0 (Linux; Android 10; SM-T720) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36",
			wantOS:    "Android",
		},
		{
			name:      "Android with older version",
			userAgent: "Mozilla/5.0 (Linux; Android 8.1.0; Nexus 5X Build/OPM7.181205.001) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.99 Mobile Safari/537.36",
			wantOS:    "Android",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)
			if result.OS != tc.wantOS {
				t.Errorf("Expected OS %s, got %s for User-Agent: %s", tc.wantOS, result.OS, tc.userAgent)
			}
			// Ensure it's not misidentified as Linux
			if result.OS == "Linux" {
				t.Errorf("Android device incorrectly identified as Linux for User-Agent: %s", tc.userAgent)
			}
		})
	}
}
