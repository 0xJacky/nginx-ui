package parser

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSimpleUserAgentParser_Parse(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name         string
		userAgent    string
		expectedInfo UserAgentInfo
	}{
		{
			name:      "Chrome on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expectedInfo: UserAgentInfo{
				Browser:    "Chrome",
				BrowserVer: "91.0",
				OS:         "Windows",
				OSVersion:  "10",
				DeviceType: "Desktop",
			},
		},
		{
			name:      "Firefox on Ubuntu",
			userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0",
			expectedInfo: UserAgentInfo{
				Browser:    "Firefox",
				BrowserVer: "89.0",
				OS:         "Ubuntu",
				OSVersion:  "",
				DeviceType: "Desktop",
			},
		},
		{
			name:      "Safari on iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
			expectedInfo: UserAgentInfo{
				Browser:    "Safari",
				BrowserVer: "14.1",
				OS:         "iOS",
				OSVersion:  "14.6",
				DeviceType: "iPhone",
			},
		},
		{
			name:      "Safari on iPad",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
			expectedInfo: UserAgentInfo{
				Browser:    "Safari",
				BrowserVer: "14.1",
				OS:         "iOS",
				OSVersion:  "14.6",
				DeviceType: "iPad",
			},
		},
		{
			name:      "WeChat Browser",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.7(0x18000733) NetType/WIFI Language/zh_CN",
			expectedInfo: UserAgentInfo{
				Browser:    "WeChat",
				BrowserVer: "8.0",
				OS:         "iOS",
				OSVersion:  "14.6",
				DeviceType: "iPhone",
			},
		},
		{
			name:      "Chrome on Android Phone",
			userAgent: "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			expectedInfo: UserAgentInfo{
				Browser:    "Chrome",
				BrowserVer: "91.0",
				OS:         "Android",
				OSVersion:  "11",
				DeviceType: "Mobile",
			},
		},
		{
			name:      "Chrome on Android Tablet",
			userAgent: "Mozilla/5.0 (Linux; Android 11; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36",
			expectedInfo: UserAgentInfo{
				Browser:    "Chrome",
				BrowserVer: "91.0",
				OS:         "Android",
				OSVersion:  "11",
				DeviceType: "Tablet",
			},
		},
		{
			name:      "Edge Browser",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
			expectedInfo: UserAgentInfo{
				Browser:    "Edge",
				BrowserVer: "91.0",
				OS:         "Windows",
				OSVersion:  "10",
				DeviceType: "Desktop",
			},
		},
		{
			name:      "UC Browser",
			userAgent: "Mozilla/5.0 (Linux; U; Android 8.1.0; zh-CN; EML-AL00 Build/HUAWEIEML-AL00) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/57.0.2987.108 UCBrowser/11.9.4.974 Mobile Safari/537.36",
			expectedInfo: UserAgentInfo{
				Browser:    "UC Browser",
				BrowserVer: "11.9",
				OS:         "Android",
				OSVersion:  "8.1",
				DeviceType: "Mobile",
			},
		},
		{
			name:      "QQ Browser",
			userAgent: "Mozilla/5.0 (Linux; Android 5.0; SM-N9100 Build/LRX21V; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/53.0.2785.49 Mobile MQQBrowser/6.2 TBS/043632 Safari/537.36 MicroMessenger/6.6.1.1220(0x26060135) NetType/WIFI Language/zh_CN",
			expectedInfo: UserAgentInfo{
				Browser:    "WeChat",
				BrowserVer: "6.6",
				OS:         "Android",
				OSVersion:  "5",
				DeviceType: "Mobile",
			},
		},
		{
			name:      "Googlebot",
			userAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			expectedInfo: UserAgentInfo{
				Browser:    "Bot",
				BrowserVer: "",
				OS:         "Unknown",
				OSVersion:  "",
				DeviceType: "Bot",
			},
		},
		{
			name:      "Samsung Browser",
			userAgent: "Mozilla/5.0 (Linux; Android 9; SM-G973F) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/9.2 Chrome/67.0.3396.87 Mobile Safari/537.36",
			expectedInfo: UserAgentInfo{
				Browser:    "Samsung Browser",
				BrowserVer: "9.2",
				OS:         "Android",
				OSVersion:  "9",
				DeviceType: "Mobile",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.userAgent)
			
			assert.Equal(t, tc.expectedInfo.Browser, result.Browser, "Browser mismatch")
			assert.Equal(t, tc.expectedInfo.BrowserVer, result.BrowserVer, "Browser version mismatch")
			assert.Equal(t, tc.expectedInfo.OS, result.OS, "OS mismatch")
			assert.Equal(t, tc.expectedInfo.OSVersion, result.OSVersion, "OS version mismatch")
			assert.Equal(t, tc.expectedInfo.DeviceType, result.DeviceType, "Device type mismatch")
		})
	}
}

func TestSimpleUserAgentParser_IsBot(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name      string
		userAgent string
		expected  bool
	}{
		{
			name:      "Googlebot",
			userAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			expected:  true,
		},
		{
			name:      "Bingbot",
			userAgent: "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
			expected:  true,
		},
		{
			name:      "Regular Chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.IsBot(tc.userAgent)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSimpleUserAgentParser_IsMobile(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name      string
		userAgent string
		expected  bool
	}{
		{
			name:      "iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expected:  true,
		},
		{
			name:      "Android Phone",
			userAgent: "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 Chrome/91.0.4472.120 Mobile Safari/537.36",
			expected:  true,
		},
		{
			name:      "Desktop Chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
			expected:  false,
		},
		{
			name:      "iPad",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expected:  false, // iPad should be tablet, not mobile
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.IsMobile(tc.userAgent)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSimpleUserAgentParser_IsTablet(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name      string
		userAgent string
		expected  bool
	}{
		{
			name:      "iPad",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expected:  true,
		},
		{
			name:      "Android Tablet",
			userAgent: "Mozilla/5.0 (Linux; Android 11; SM-T870) AppleWebKit/537.36 Chrome/91.0.4472.120 Safari/537.36",
			expected:  true,
		},
		{
			name:      "iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expected:  false,
		},
		{
			name:      "Desktop",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.IsTablet(tc.userAgent)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCachedUserAgentParser_Enhanced(t *testing.T) {
	baseParser := NewSimpleUserAgentParser()
	cachedParser := NewCachedUserAgentParser(baseParser, 10)

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36"

	// First call should parse and cache
	result1 := cachedParser.Parse(userAgent)
	assert.Equal(t, "Chrome", result1.Browser)

	// Second call should return cached result
	result2 := cachedParser.Parse(userAgent)
	assert.Equal(t, result1, result2)

	// Check cache stats
	size, maxSize := cachedParser.GetCacheStats()
	assert.Equal(t, 1, size)
	assert.Equal(t, 10, maxSize)

	// Clear cache
	cachedParser.ClearCache()
	size, _ = cachedParser.GetCacheStats()
	assert.Equal(t, 0, size)
}

func TestSimpleUserAgentParser_GetSimpleDeviceType(t *testing.T) {
	parser := NewSimpleUserAgentParser()

	testCases := []struct {
		name      string
		userAgent string
		expected  string
	}{
		{
			name:      "iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expected:  "Mobile",
		},
		{
			name:      "iPad",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15",
			expected:  "Tablet",
		},
		{
			name:      "Bot",
			userAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			expected:  "Bot",
		},
		{
			name:      "Desktop",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			expected:  "Desktop",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.GetSimpleDeviceType(tc.userAgent)
			assert.Equal(t, tc.expected, result)
		})
	}
}