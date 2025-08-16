package nginx_log

import (
	"testing"
)

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
		{
			name:            "Edge with correct version",
			userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
			expectedBrowser: "Edge",
			expectedVersion: "91.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.Browser != tc.expectedBrowser {
				t.Errorf("expected browser %q, got %q", tc.expectedBrowser, result.Browser)
			}
			if result.BrowserVer != tc.expectedVersion {
				t.Errorf("expected version %q, got %q", tc.expectedVersion, result.BrowserVer)
			}
		})
	}
}

func TestOSVersionParsing(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name           string
		userAgent      string
		expectedOS     string
		expectedOSVer  string
	}{
		{
			name:           "Windows 10",
			userAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			expectedOS:     "Windows 10",
			expectedOSVer:  "10.0",
		},
		{
			name:           "macOS",
			userAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			expectedOS:     "macOS",
			expectedOSVer:  "10.15.7",
		},
		{
			name:           "iOS",
			userAgent:      "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expectedOS:     "iOS",
			expectedOSVer:  "14.6",
		},
		{
			name:           "Android",
			userAgent:      "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36",
			expectedOS:     "Android",
			expectedOSVer:  "11",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.OS != tc.expectedOS {
				t.Errorf("expected OS %q, got %q", tc.expectedOS, result.OS)
			}
			if result.OSVersion != tc.expectedOSVer {
				t.Errorf("expected OS version %q, got %q", tc.expectedOSVer, result.OSVersion)
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
			name:               "Desktop Chrome",
			userAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expectedDeviceType: "Desktop",
		},
		{
			name:               "iPhone Safari",
			userAgent:          "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expectedDeviceType: "iPhone",
		},
		{
			name:               "iPad Safari",
			userAgent:          "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expectedDeviceType: "iPad",
		},
		{
			name:               "Android Mobile",
			userAgent:          "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			expectedDeviceType: "Mobile",
		},
		{
			name:               "Android Tablet",
			userAgent:          "Mozilla/5.0 (Linux; Android 11; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36",
			expectedDeviceType: "Tablet",
		},
		{
			name:               "Bot",
			userAgent:          "Googlebot/2.1 (+http://www.google.com/bot.html)",
			expectedDeviceType: "Bot",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.DeviceType != tc.expectedDeviceType {
				t.Errorf("expected device type %q, got %q", tc.expectedDeviceType, result.DeviceType)
			}
		})
	}
}

func TestAndroidOSParsing(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name            string
		userAgent       string
		expectedOS      string
		expectedOSVer   string
		expectedDevice  string
	}{
		{
			name:            "Samsung Galaxy S21",
			userAgent:       "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			expectedOS:      "Android",
			expectedOSVer:   "11",
			expectedDevice:  "Mobile",
		},
		{
			name:            "Samsung Galaxy Tab",
			userAgent:       "Mozilla/5.0 (Linux; Android 11; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36",
			expectedOS:      "Android",
			expectedOSVer:   "11",
			expectedDevice:  "Tablet",
		},
		{
			name:            "Pixel 5",
			userAgent:       "Mozilla/5.0 (Linux; Android 12; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Mobile Safari/537.36",
			expectedOS:      "Android",
			expectedOSVer:   "12",
			expectedDevice:  "Mobile",
		},
		{
			name:            "Android 10",
			userAgent:       "Mozilla/5.0 (Linux; Android 10; SM-A505FN) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.149 Mobile Safari/537.36",
			expectedOS:      "Android",
			expectedOSVer:   "10",
			expectedDevice:  "Mobile",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.OS != tc.expectedOS {
				t.Errorf("expected OS %q, got %q", tc.expectedOS, result.OS)
			}
			if result.OSVersion != tc.expectedOSVer {
				t.Errorf("expected OS version %q, got %q", tc.expectedOSVer, result.OSVersion)
			}
			if result.DeviceType != tc.expectedDevice {
				t.Errorf("expected device type %q, got %q", tc.expectedDevice, result.DeviceType)
			}
		})
	}
}

func TestRealWorldAndroidUserAgents(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name            string
		userAgent       string
		expectedOS      string
		expectedOSVer   string
		expectedDevice  string
		expectedBrowser string
	}{
		{
			name:            "Samsung Internet on Galaxy S20",
			userAgent:       "Mozilla/5.0 (Linux; Android 10; SM-G981B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/12.0 Chrome/79.0.3945.136 Mobile Safari/537.36",
			expectedOS:      "Android",
			expectedOSVer:   "10",
			expectedDevice:  "Mobile",
			expectedBrowser: "Samsung Internet",
		},
		{
			name:            "Chrome on OnePlus",
			userAgent:       "Mozilla/5.0 (Linux; Android 11; OnePlus 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Mobile Safari/537.36",
			expectedOS:      "Android",
			expectedOSVer:   "11",
			expectedDevice:  "Mobile",
			expectedBrowser: "Chrome",
		},
		{
			name:            "Firefox on Android",
			userAgent:       "Mozilla/5.0 (Mobile; rv:89.0) Gecko/89.0 Firefox/89.0",
			expectedOS:      "Android",
			expectedOSVer:   "",
			expectedDevice:  "Mobile",
			expectedBrowser: "Firefox",
		},
		{
			name:            "Edge on Android",
			userAgent:       "Mozilla/5.0 (Linux; Android 10; HD1913) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.101 Mobile Safari/537.36 EdgA/46.3.4.5155",
			expectedOS:      "Android",
			expectedOSVer:   "10",
			expectedDevice:  "Mobile",
			expectedBrowser: "Edge",
		},
		{
			name:            "Opera on Android",
			userAgent:       "Mozilla/5.0 (Linux; Android 10; SM-A205U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.88 Mobile Safari/537.36 OPR/64.2.3282.60455",
			expectedOS:      "Android",
			expectedOSVer:   "10",
			expectedDevice:  "Mobile",
			expectedBrowser: "Opera",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)

			if result.OS != tc.expectedOS {
				t.Errorf("expected OS %q, got %q", tc.expectedOS, result.OS)
			}
			if result.OSVersion != tc.expectedOSVer && tc.expectedOSVer != "" {
				t.Errorf("expected OS version %q, got %q", tc.expectedOSVer, result.OSVersion)
			}
			if result.DeviceType != tc.expectedDevice {
				t.Errorf("expected device type %q, got %q", tc.expectedDevice, result.DeviceType)
			}
			if result.Browser != tc.expectedBrowser {
				t.Errorf("expected browser %q, got %q", tc.expectedBrowser, result.Browser)
			}
		})
	}
}