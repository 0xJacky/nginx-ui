package nginx_log

import (
	"regexp"
	"strings"
)

// SimpleUserAgentParser implements UserAgentParser with regex-based parsing
type SimpleUserAgentParser struct {
	browserPatterns map[string]*regexp.Regexp
	osPatterns      map[string]*regexp.Regexp
	devicePatterns  map[string]*regexp.Regexp
}

// NewSimpleUserAgentParser creates a new simple user agent parser
func NewSimpleUserAgentParser() *SimpleUserAgentParser {
	return &SimpleUserAgentParser{
		browserPatterns: initBrowserPatterns(),
		osPatterns:      initOSPatterns(),
		devicePatterns:  initDevicePatterns(),
	}
}

// Parse parses a user agent string and returns structured information
func (p *SimpleUserAgentParser) Parse(userAgent string) UserAgentInfo {
	if userAgent == "" || userAgent == "-" {
		return UserAgentInfo{}
	}

	info := UserAgentInfo{}

	// Parse browser information
	info.Browser, info.BrowserVer = p.parseBrowser(userAgent)

	// Parse OS information
	info.OS, info.OSVersion = p.parseOS(userAgent)

	// Parse device type
	info.DeviceType = p.parseDeviceType(userAgent)

	return info
}

// parseBrowser extracts browser name and version
func (p *SimpleUserAgentParser) parseBrowser(userAgent string) (browser, version string) {
	// Try each browser pattern
	for name, pattern := range p.browserPatterns {
		if matches := pattern.FindStringSubmatch(userAgent); len(matches) >= 2 {
			browser = name
			if len(matches) >= 3 {
				// Combine major and minor version: matches[1].matches[2]
				version = matches[1] + "." + matches[2]
			} else if len(matches) >= 2 {
				// Only major version available
				version = matches[1]
			}
			return
		}
	}

	return "Unknown", ""
}

// parseOS extracts operating system name and version
func (p *SimpleUserAgentParser) parseOS(userAgent string) (os, version string) {
	// Check specific OS patterns in order of specificity
	osOrder := []string{
		"iOS",     // iOS must come before macOS to avoid false matches
		"Android", // Android must come before Linux since Android contains "Linux"
		"Windows", "macOS", 
		"Ubuntu", "CentOS", "Debian", "Red Hat", "Fedora", "SUSE", "Linux",
	}

	for _, name := range osOrder {
		if pattern, exists := p.osPatterns[name]; exists {
			if matches := pattern.FindStringSubmatch(userAgent); len(matches) >= 1 {
				os = name
				if len(matches) >= 3 && matches[2] != "" {
					// Two capture groups: major.minor version
					version = matches[1] + "." + matches[2]
				} else if len(matches) >= 2 {
					// One capture group: version
					if name == "Android" {
						// For Android, add .0 if no minor version
						version = matches[1] + ".0"
					} else {
						version = matches[1]
					}
				}
				return
			}
		}
	}

	return "Unknown", ""
}

// parseDeviceType determines the device type
func (p *SimpleUserAgentParser) parseDeviceType(userAgent string) string {
	userAgent = strings.ToLower(userAgent)

	// Check for specific device types in order of priority
	// Bot detection first
	if p.devicePatterns["Bot"].MatchString(userAgent) {
		return "Bot"
	}

	// Apple devices (specific models first)
	if p.devicePatterns["iPhone"].MatchString(userAgent) {
		return "iPhone"
	}
	if p.devicePatterns["iPad"].MatchString(userAgent) {
		return "iPad"
	}
	if p.devicePatterns["iPod"].MatchString(userAgent) {
		return "iPod"
	}

	// Mobile detection (Android Mobile and other mobile devices)
	if p.devicePatterns["Mobile"].MatchString(userAgent) ||
		(strings.Contains(userAgent, "android") && strings.Contains(userAgent, "mobile")) {
		return "Mobile"
	}

	// Tablet detection (Android tablets and other tablets)
	if p.devicePatterns["Tablet"].MatchString(userAgent) ||
		(strings.Contains(userAgent, "android") && !strings.Contains(userAgent, "mobile")) {
		return "Tablet"
	}

	// Check other device types
	for deviceType, pattern := range p.devicePatterns {
		if deviceType != "Bot" && deviceType != "Mobile" && deviceType != "Tablet" && deviceType != "Desktop" &&
			deviceType != "iPhone" && deviceType != "iPad" && deviceType != "iPod" {
			if pattern.MatchString(userAgent) {
				return deviceType
			}
		}
	}

	return "Desktop"
}

// initBrowserPatterns initializes browser detection patterns
func initBrowserPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"Chrome":            regexp.MustCompile(`(?i)chrome[\/\s](\d+)\.(\d+)`),
		"Firefox":           regexp.MustCompile(`(?i)firefox[\/\s](\d+)\.(\d+)`),
		"Safari":            regexp.MustCompile(`(?i)version[\/\s](\d+)\.(\d+).*safari`),
		"Edge":              regexp.MustCompile(`(?i)edg[\/\s](\d+)\.(\d+)`),
		"Internet Explorer": regexp.MustCompile(`(?i)msie[\/\s](\d+)\.(\d+)`),
		"Opera":             regexp.MustCompile(`(?i)opera[\/\s](\d+)\.(\d+)`),
		"Brave":             regexp.MustCompile(`(?i)brave[\/\s](\d+)\.(\d+)`),
		"Vivaldi":           regexp.MustCompile(`(?i)vivaldi[\/\s](\d+)\.(\d+)`),
		"UC Browser":        regexp.MustCompile(`(?i)ucbrowser[\/\s](\d+)\.(\d+)`),
		"Samsung Browser":   regexp.MustCompile(`(?i)samsungbrowser[\/\s](\d+)\.(\d+)`),
		"Yandex":            regexp.MustCompile(`(?i)yabrowser[\/\s](\d+)\.(\d+)`),
		"QQ Browser":        regexp.MustCompile(`(?i)qqbrowser[\/\s](\d+)\.(\d+)`),
		"Sogou Explorer":    regexp.MustCompile(`(?i)se[\/\s](\d+)\.(\d+)`),
		"360 Browser":       regexp.MustCompile(`(?i)360se[\/\s](\d+)\.(\d+)`),
		"Maxthon":           regexp.MustCompile(`(?i)maxthon[\/\s](\d+)\.(\d+)`),
		"Baidu Browser":     regexp.MustCompile(`(?i)baidubrowser[\/\s](\d+)\.(\d+)`),
		"WeChat":            regexp.MustCompile(`(?i)micromessenger[\/\s](\d+)\.(\d+)`),
		"QQ":                regexp.MustCompile(`(?i)qq[\/\s](\d+)\.(\d+)`),
		"DingTalk":          regexp.MustCompile(`(?i)dingtalk[\/\s](\d+)\.(\d+)`),
		"Alipay":            regexp.MustCompile(`(?i)alipayclient[\/\s](\d+)\.(\d+)`),
	}
}

// initOSPatterns initializes operating system detection patterns
func initOSPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"Windows":   regexp.MustCompile(`(?i)windows`),
		"macOS":     regexp.MustCompile(`(?i)mac os x|macos|darwin`),
		"iOS":       regexp.MustCompile(`(?i)(?:iphone|ipad|ipod).*?(?:iphone )?os (\d+)[_\.](\d+)`),
		"Android":   regexp.MustCompile(`(?i)android (\d+)(?:\.(\d+))?`),
		"Ubuntu":    regexp.MustCompile(`(?i)ubuntu[\/\s](\d+)\.(\d+)`),
		"CentOS":    regexp.MustCompile(`(?i)centos[\/\s](\d+)`),
		"Debian":    regexp.MustCompile(`(?i)debian`),
		"Red Hat":   regexp.MustCompile(`(?i)red hat`),
		"Fedora":    regexp.MustCompile(`(?i)fedora[\/\s](\d+)`),
		"SUSE":      regexp.MustCompile(`(?i)suse`),
		"Linux":     regexp.MustCompile(`(?i)linux`),
		"FreeBSD":   regexp.MustCompile(`(?i)freebsd`),
		"OpenBSD":   regexp.MustCompile(`(?i)openbsd`),
		"NetBSD":    regexp.MustCompile(`(?i)netbsd`),
		"Unix":      regexp.MustCompile(`(?i)unix`),
		"Chrome OS": regexp.MustCompile(`(?i)cros`),
	}
}

// initDevicePatterns initializes device type detection patterns
func initDevicePatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"iPhone":        regexp.MustCompile(`(?i)iphone`),
		"iPad":          regexp.MustCompile(`(?i)ipad`),
		"iPod":          regexp.MustCompile(`(?i)ipod`),
		"Mobile":        regexp.MustCompile(`(?i)mobile|phone|blackberry|windows phone|palm|symbian`),
		"Tablet":        regexp.MustCompile(`(?i)tablet|kindle|silk`),
		"TV":            regexp.MustCompile(`(?i)smart-?tv|tv|roku|chromecast|apple.?tv|xbox|playstation|nintendo`),
		"Bot":           regexp.MustCompile(`(?i)bot|crawl|spider|scraper|parser|checker|monitoring|curl|wget|python|java|go-http|okhttp`),
		"Smart Speaker": regexp.MustCompile(`(?i)alexa|google.?home|echo`),
		"Game Console":  regexp.MustCompile(`(?i)xbox|playstation|nintendo|psp|vita`),
		"Wearable":      regexp.MustCompile(`(?i)watch|wearable`),
		"Desktop":       regexp.MustCompile(`.*`), // Default fallback
	}
}

// MockUserAgentParser is a mock implementation for testing
type MockUserAgentParser struct {
	responses map[string]UserAgentInfo
}

// NewMockUserAgentParser creates a new mock user agent parser
func NewMockUserAgentParser() *MockUserAgentParser {
	return &MockUserAgentParser{
		responses: map[string]UserAgentInfo{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36": {
				Browser:    "Chrome",
				BrowserVer: "91.0",
				OS:         "Windows 10",
				OSVersion:  "10.0",
				DeviceType: "Desktop",
			},
			"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X)": {
				Browser:    "Safari",
				BrowserVer: "14.0",
				OS:         "iOS",
				OSVersion:  "14.6",
				DeviceType: "Mobile",
			},
		},
	}
}

// Parse returns mock user agent information for testing
func (m *MockUserAgentParser) Parse(userAgent string) UserAgentInfo {
	if info, exists := m.responses[userAgent]; exists {
		return info
	}
	return UserAgentInfo{
		Browser:    "Unknown",
		OS:         "Unknown",
		DeviceType: "Desktop",
	}
}

// AddResponse adds a mock response for testing
func (m *MockUserAgentParser) AddResponse(userAgent string, info UserAgentInfo) {
	m.responses[userAgent] = info
}
