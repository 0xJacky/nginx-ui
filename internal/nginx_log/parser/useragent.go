package parser

import (
	"regexp"
	"strings"
)

// SimpleUserAgentParser implements a lightweight user agent parser
type SimpleUserAgentParser struct {
	browserPatterns []browserPattern
	osPatterns      []osPattern
	devicePatterns  []devicePattern
}

type browserPattern struct {
	name    string
	pattern *regexp.Regexp
	version *regexp.Regexp
}

type osPattern struct {
	name    string
	pattern *regexp.Regexp
	version *regexp.Regexp
}

type devicePattern struct {
	name    string
	pattern *regexp.Regexp
}

// NewSimpleUserAgentParser creates a new simple user agent parser
func NewSimpleUserAgentParser() *SimpleUserAgentParser {
	return &SimpleUserAgentParser{
		browserPatterns: initBrowserPatterns(),
		osPatterns:      initOSPatterns(),
		devicePatterns:  initDevicePatterns(),
	}
}

func initBrowserPatterns() []browserPattern {
	return []browserPattern{
		// Bot detection (highest priority)
		{
			name:    "Bot",
			pattern: regexp.MustCompile(`(?i)bot|crawler|spider|crawl|slurp|sohu-search|lycos|robozilla|googlebot|bingbot|facebookexternalhit|twitterbot|whatsapp|telegrambot|applebot|linkedinbot|pinterest|yandexbot|baiduspider|360spider|sogou|bytedance|tiktok`),
			version: nil,
		},

		// Mobile Apps and Special Browsers (high priority)
		{
			name:    "WeChat",
			pattern: regexp.MustCompile(`(?i)micromessenger`),
			version: regexp.MustCompile(`(?i)micromessenger/(\d+\.\d+)`),
		},
		{
			name:    "QQ",
			pattern: regexp.MustCompile(`(?i)qq/(\d+\.\d+)`),
			version: regexp.MustCompile(`(?i)qq/(\d+\.\d+)`),
		},
		{
			name:    "DingTalk",
			pattern: regexp.MustCompile(`(?i)dingtalk`),
			version: regexp.MustCompile(`(?i)dingtalk/(\d+\.\d+)`),
		},
		{
			name:    "Alipay",
			pattern: regexp.MustCompile(`(?i)alipayclient`),
			version: regexp.MustCompile(`(?i)alipayclient/(\d+\.\d+)`),
		},
		{
			name:    "TikTok",
			pattern: regexp.MustCompile(`(?i)musically_`),
			version: regexp.MustCompile(`(?i)musically_(\d+\.\d+)`),
		},

		// Chinese Browsers
		{
			name:    "360 Browser",
			pattern: regexp.MustCompile(`(?i)360se|qihoobrowser`),
			version: regexp.MustCompile(`(?i)360se/(\d+\.\d+)|qihoobrowser/(\d+\.\d+)`),
		},
		{
			name:    "QQ Browser",
			pattern: regexp.MustCompile(`(?i)qqbrowser`),
			version: regexp.MustCompile(`(?i)qqbrowser/(\d+\.\d+)`),
		},
		{
			name:    "UC Browser",
			pattern: regexp.MustCompile(`(?i)ucbrowser|uc browser`),
			version: regexp.MustCompile(`(?i)ucbrowser/(\d+\.\d+)`),
		},
		{
			name:    "Sogou Explorer",
			pattern: regexp.MustCompile(`(?i)se |metasr`),
			version: regexp.MustCompile(`(?i)se (\d+\.\d+)|metasr (\d+\.\d+)`),
		},
		{
			name:    "Baidu Browser",
			pattern: regexp.MustCompile(`(?i)baidubrowser|bidubrowser`),
			version: regexp.MustCompile(`(?i)baidubrowser/(\d+\.\d+)|bidubrowser/(\d+\.\d+)`),
		},
		{
			name:    "Maxthon",
			pattern: regexp.MustCompile(`(?i)maxthon`),
			version: regexp.MustCompile(`(?i)maxthon/(\d+\.\d+)`),
		},

		// International Mobile Browsers
		{
			name:    "Samsung Browser",
			pattern: regexp.MustCompile(`(?i)samsungbrowser`),
			version: regexp.MustCompile(`(?i)samsungbrowser/(\d+\.\d+)`),
		},
		{
			name:    "Huawei Browser",
			pattern: regexp.MustCompile(`(?i)huaweibrowser`),
			version: regexp.MustCompile(`(?i)huaweibrowser/(\d+\.\d+)`),
		},
		{
			name:    "Xiaomi Browser",
			pattern: regexp.MustCompile(`(?i)mibrowser`),
			version: regexp.MustCompile(`(?i)mibrowser/(\d+\.\d+)`),
		},
		{
			name:    "Oppo Browser",
			pattern: regexp.MustCompile(`(?i)oppobrowser`),
			version: regexp.MustCompile(`(?i)oppobrowser/(\d+\.\d+)`),
		},
		{
			name:    "Vivo Browser",
			pattern: regexp.MustCompile(`(?i)vivobrowser`),
			version: regexp.MustCompile(`(?i)vivobrowser/(\d+\.\d+)`),
		},

		// International Browsers
		{
			name:    "Yandex",
			pattern: regexp.MustCompile(`(?i)yabrowser`),
			version: regexp.MustCompile(`(?i)yabrowser/(\d+\.\d+)`),
		},
		{
			name:    "Brave",
			pattern: regexp.MustCompile(`(?i)brave`),
			version: regexp.MustCompile(`(?i)brave/(\d+\.\d+)`),
		},
		{
			name:    "Vivaldi",
			pattern: regexp.MustCompile(`(?i)vivaldi`),
			version: regexp.MustCompile(`(?i)vivaldi/(\d+\.\d+)`),
		},

		// Microsoft Browsers (Order matters - Edge before IE)
		{
			name:    "Edge",
			pattern: regexp.MustCompile(`(?i)edg/|edge/`),
			version: regexp.MustCompile(`(?i)edg?[e]?/(\d+\.\d+)`),
		},
		{
			name:    "Internet Explorer",
			pattern: regexp.MustCompile(`(?i)msie |trident.*rv:`),
			version: regexp.MustCompile(`(?i)msie (\d+\.\d+)|rv:(\d+\.\d+)`),
		},

		// Major Browsers (Order matters - Chrome variants before Chrome)
		{
			name:    "Opera",
			pattern: regexp.MustCompile(`(?i)opr/|opera/`),
			version: regexp.MustCompile(`(?i)opr/(\d+\.\d+)|version/(\d+\.\d+)`),
		},
		{
			name:    "Chrome",
			pattern: regexp.MustCompile(`(?i)chrome/`),
			version: regexp.MustCompile(`(?i)chrome/(\d+\.\d+)`),
		},
		{
			name:    "Firefox",
			pattern: regexp.MustCompile(`(?i)firefox/`),
			version: regexp.MustCompile(`(?i)firefox/(\d+\.\d+)`),
		},
		{
			name:    "Safari",
			pattern: regexp.MustCompile(`(?i)safari/`),
			version: regexp.MustCompile(`(?i)version/(\d+\.\d+)`),
		},

		// Other/Legacy Browsers
		{
			name:    "NetFront",
			pattern: regexp.MustCompile(`(?i)netfront`),
			version: regexp.MustCompile(`(?i)netfront/(\d+\.\d+)`),
		},
		{
			name:    "Konqueror",
			pattern: regexp.MustCompile(`(?i)konqueror`),
			version: regexp.MustCompile(`(?i)konqueror/(\d+\.\d+)`),
		},
	}
}

func initOSPatterns() []osPattern {
	return []osPattern{
		// Mobile OS (highest priority)
		{
			name:    "iOS",
			pattern: regexp.MustCompile(`(?i)iPhone OS|OS (\d+_\d+)|iPad; OS|iPod.*OS|iPhone.*OS`),
			version: regexp.MustCompile(`(?i)OS (\d+[_\d]*)`),
		},
		{
			name:    "Android",
			pattern: regexp.MustCompile(`(?i)android`),
			version: regexp.MustCompile(`(?i)android (\d+\.?\d*\.?\d*)`),
		},

		// Desktop OS
		{
			name:    "Windows",
			pattern: regexp.MustCompile(`(?i)windows`),
			version: regexp.MustCompile(`(?i)windows nt (\d+\.?\d*)`),
		},
		{
			name:    "macOS",
			pattern: regexp.MustCompile(`(?i)mac os x|macintosh|intel mac`),
			version: regexp.MustCompile(`(?i)mac os x (\d+[_\d]*)`),
		},

		// Linux Distributions
		{
			name:    "Ubuntu",
			pattern: regexp.MustCompile(`(?i)ubuntu`),
			version: regexp.MustCompile(`(?i)ubuntu[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "CentOS",
			pattern: regexp.MustCompile(`(?i)centos`),
			version: regexp.MustCompile(`(?i)centos[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "Red Hat",
			pattern: regexp.MustCompile(`(?i)red.*hat|rhel`),
			version: regexp.MustCompile(`(?i)red.*hat[\/\s]*(\d+\.?\d*\.?\d*)|rhel[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "Debian",
			pattern: regexp.MustCompile(`(?i)debian`),
			version: regexp.MustCompile(`(?i)debian[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "Fedora",
			pattern: regexp.MustCompile(`(?i)fedora`),
			version: regexp.MustCompile(`(?i)fedora[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "SUSE",
			pattern: regexp.MustCompile(`(?i)suse|opensuse`),
			version: regexp.MustCompile(`(?i)suse[\/\s]*(\d+\.?\d*\.?\d*)|opensuse[\/\s]*(\d+\.?\d*\.?\d*)`),
		},

		// Other Unix-like
		{
			name:    "FreeBSD",
			pattern: regexp.MustCompile(`(?i)freebsd`),
			version: regexp.MustCompile(`(?i)freebsd (\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "OpenBSD",
			pattern: regexp.MustCompile(`(?i)openbsd`),
			version: regexp.MustCompile(`(?i)openbsd (\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "NetBSD",
			pattern: regexp.MustCompile(`(?i)netbsd`),
			version: regexp.MustCompile(`(?i)netbsd (\d+\.?\d*\.?\d*)`),
		},

		// Generic Linux fallback
		{
			name:    "Linux",
			pattern: regexp.MustCompile(`(?i)linux|x11`),
			version: nil,
		},

		// Other/Legacy OS
		{
			name:    "Chrome OS",
			pattern: regexp.MustCompile(`(?i)cros`),
			version: regexp.MustCompile(`(?i)cros (\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "Windows Phone",
			pattern: regexp.MustCompile(`(?i)windows phone`),
			version: regexp.MustCompile(`(?i)windows phone (\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "BlackBerry",
			pattern: regexp.MustCompile(`(?i)blackberry|bb10`),
			version: regexp.MustCompile(`(?i)blackberry[\/\s]*(\d+\.?\d*\.?\d*)|bb10[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
		{
			name:    "Symbian",
			pattern: regexp.MustCompile(`(?i)symbian|s60`),
			version: regexp.MustCompile(`(?i)symbian[\/\s]*(\d+\.?\d*\.?\d*)|s60[\/\s]*(\d+\.?\d*\.?\d*)`),
		},
	}
}

func initDevicePatterns() []devicePattern {
	return []devicePattern{
		// Bots (highest priority)
		{
			name:    "Bot",
			pattern: regexp.MustCompile(`(?i)bot|crawler|spider|crawl|slurp|sohu-search|lycos|robozilla|googlebot|bingbot|facebookexternalhit|twitterbot|whatsapp|telegrambot|applebot|linkedinbot|pinterest|yandexbot|baiduspider|360spider|sogou|bytedance|scraper`),
		},

		// Apple Devices (specific models first)
		{
			name:    "iPhone",
			pattern: regexp.MustCompile(`(?i)iphone`),
		},
		{
			name:    "iPad",
			pattern: regexp.MustCompile(`(?i)ipad`),
		},
		{
			name:    "iPod",
			pattern: regexp.MustCompile(`(?i)ipod`),
		},
		{
			name:    "Apple Watch",
			pattern: regexp.MustCompile(`(?i)watch.*os`),
		},
		{
			name:    "Apple TV",
			pattern: regexp.MustCompile(`(?i)apple.*tv`),
		},

		// Gaming Consoles
		{
			name:    "PlayStation",
			pattern: regexp.MustCompile(`(?i)playstation|ps[345]|psvita`),
		},
		{
			name:    "Xbox",
			pattern: regexp.MustCompile(`(?i)xbox`),
		},
		{
			name:    "Nintendo",
			pattern: regexp.MustCompile(`(?i)nintendo|wii|3ds|switch`),
		},

		// Smart TVs and Streaming Devices
		{
			name:    "Smart TV",
			pattern: regexp.MustCompile(`(?i)smart.*tv|smarttv|hbbtv|netcast|roku|webos|tizen|android.*tv`),
		},
		{
			name:    "Chromecast",
			pattern: regexp.MustCompile(`(?i)chromecast`),
		},

		// Tablets (before Mobile for proper detection)
		{
			name:    "Tablet",
			pattern: regexp.MustCompile(`(?i)tablet|ipad|kindle|nook|playbook|touchpad|xoom|sch-i800|gt-p1000|sgh-t849|shw-m180s|a1_07|bntv250a|mid7015|mid7012`),
		},

		// Mobile Phones (Android and others)
		{
			name:    "Mobile",
			pattern: regexp.MustCompile(`(?i)mobile|phone|iphone|android.*mobile|blackberry|bb10|windows phone|iemobile|palm|webos|symbian|maemo|fennec|minimo|pda|pocket|psp|smartphone|mobileexplorer|htc|samsung|lg|motorola|sony|nokia|huawei|xiaomi|oppo|vivo|oneplus`),
		},

		// Wearables
		{
			name:    "Wearable",
			pattern: regexp.MustCompile(`(?i)watch|wearable|fitbit|gear`),
		},

		// IoT and Other Devices
		{
			name:    "IoT Device",
			pattern: regexp.MustCompile(`(?i)alexa|echo|iot|raspberry|arduino`),
		},

		// E-readers
		{
			name:    "E-Reader",
			pattern: regexp.MustCompile(`(?i)kindle|nook|kobo|pocketbook`),
		},

		// Default fallback (must be last)
		{
			name:    "Desktop",
			pattern: regexp.MustCompile(`.*`),
		},
	}
}

// Parse parses a user agent string and returns detailed information
func (p *SimpleUserAgentParser) Parse(userAgent string) UserAgentInfo {
	if userAgent == "" || userAgent == "-" {
		return UserAgentInfo{
			Browser:    "Unknown",
			BrowserVer: "",
			OS:         "Unknown",
			OSVersion:  "",
			DeviceType: "Desktop",
		}
	}

	info := UserAgentInfo{
		Browser:    "Unknown",
		BrowserVer: "",
		OS:         "Unknown",
		OSVersion:  "",
		DeviceType: "Desktop",
	}

	// Parse browser information
	for _, bp := range p.browserPatterns {
		if bp.pattern.MatchString(userAgent) {
			info.Browser = bp.name
			if bp.version != nil {
				if matches := bp.version.FindStringSubmatch(userAgent); len(matches) > 1 {
					info.BrowserVer = matches[1]
				}
			}
			break
		}
	}

	// Parse OS information
	for _, op := range p.osPatterns {
		if op.pattern.MatchString(userAgent) {
			info.OS = op.name
			if op.version != nil {
				if matches := op.version.FindStringSubmatch(userAgent); len(matches) > 1 {
					version := matches[1]
					// Clean up version string
					version = strings.ReplaceAll(version, "_", ".")
					info.OSVersion = version
				}
			}
			break
		}
	}

	// Parse device type with improved logic
	for _, dp := range p.devicePatterns {
		if dp.pattern.MatchString(userAgent) {
			info.DeviceType = dp.name
			if dp.name != "Desktop" { // Don't break on Desktop (fallback)
				break
			}
		}
	}

	// Post-processing to fix common issues
	info = p.postProcessInfo(info, userAgent)

	return info
}

// postProcessInfo applies post-processing rules to improve detection accuracy
func (p *SimpleUserAgentParser) postProcessInfo(info UserAgentInfo, userAgent string) UserAgentInfo {
	userAgentLower := strings.ToLower(userAgent)
	
	// Fix version extraction for some browsers
	if info.BrowserVer == "" {
		info.BrowserVer = p.extractVersion(info.Browser, userAgent)
	}
	
	// Special handling for mobile vs tablet detection
	if info.DeviceType == "Mobile" {
		if strings.Contains(userAgentLower, "ipad") || 
		   strings.Contains(userAgentLower, "tablet") ||
		   (strings.Contains(userAgentLower, "android") && !strings.Contains(userAgentLower, "mobile")) {
			info.DeviceType = "Tablet"
		}
	}
	
	// Fix Android tablet detection when detected as Desktop
	if info.DeviceType == "Desktop" && strings.Contains(userAgentLower, "android") && !strings.Contains(userAgentLower, "mobile") {
		info.DeviceType = "Tablet"
	}
	
	// Clean up OS versions
	if info.OSVersion != "" {
		info.OSVersion = p.cleanVersion(info.OSVersion)
	}
	
	// Fix browser names for specific cases
	info.Browser = p.fixBrowserName(info.Browser, userAgent)
	
	return info
}

// extractVersion extracts version from user agent for specific browsers
func (p *SimpleUserAgentParser) extractVersion(browser, userAgent string) string {
	switch browser {
	case "WeChat":
		if matches := regexp.MustCompile(`(?i)micromessenger/(\d+\.\d+\.\d+)`).FindStringSubmatch(userAgent); len(matches) > 1 {
			return matches[1]
		}
	case "QQ":
		if matches := regexp.MustCompile(`(?i)qq/(\d+\.\d+\.\d+)`).FindStringSubmatch(userAgent); len(matches) > 1 {
			return matches[1]
		}
	case "Alipay":
		if matches := regexp.MustCompile(`(?i)alipayclient/(\d+\.\d+\.\d+)`).FindStringSubmatch(userAgent); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// cleanVersion cleans up version strings
func (p *SimpleUserAgentParser) cleanVersion(version string) string {
	// Replace underscores with dots for iOS versions
	version = strings.ReplaceAll(version, "_", ".")
	
	// Only trim trailing .0 patterns, not individual zeros
	for strings.HasSuffix(version, ".0") {
		version = strings.TrimSuffix(version, ".0")
	}
	
	return version
}

// fixBrowserName applies corrections to browser names
func (p *SimpleUserAgentParser) fixBrowserName(browser, userAgent string) string {
	userAgentLower := strings.ToLower(userAgent)
	
	// Distinguish between different Chrome-based browsers
	if browser == "Chrome" {
		if strings.Contains(userAgentLower, "edg/") {
			return "Edge"
		}
		if strings.Contains(userAgentLower, "opr/") {
			return "Opera"
		}
		if strings.Contains(userAgentLower, "samsungbrowser") {
			return "Samsung Browser"
		}
	}
	
	return browser
}

// IsBot returns true if the user agent appears to be a bot/crawler
func (p *SimpleUserAgentParser) IsBot(userAgent string) bool {
	info := p.Parse(userAgent)
	return info.Browser == "Bot" || info.DeviceType == "Bot"
}

// IsMobile returns true if the user agent appears to be from a mobile device
func (p *SimpleUserAgentParser) IsMobile(userAgent string) bool {
	info := p.Parse(userAgent)
	return info.DeviceType == "Mobile" || info.DeviceType == "iPhone"
}

// IsTablet returns true if the user agent appears to be from a tablet device
func (p *SimpleUserAgentParser) IsTablet(userAgent string) bool {
	info := p.Parse(userAgent)
	return info.DeviceType == "Tablet" || info.DeviceType == "iPad"
}

// GetSimpleDeviceType returns a simplified device type (Mobile, Tablet, Desktop, Bot)
func (p *SimpleUserAgentParser) GetSimpleDeviceType(userAgent string) string {
	info := p.Parse(userAgent)
	
	switch info.DeviceType {
	case "iPhone", "Mobile":
		return "Mobile"
	case "iPad", "Tablet":
		return "Tablet"
	case "Bot":
		return "Bot"
	default:
		return "Desktop"
	}
}

// CachedUserAgentParser provides caching for parsed user agents
type CachedUserAgentParser struct {
	parser UserAgentParser
	cache  map[string]UserAgentInfo
	maxSize int
}

// NewCachedUserAgentParser creates a cached user agent parser
func NewCachedUserAgentParser(parser UserAgentParser, maxSize int) *CachedUserAgentParser {
	if maxSize <= 0 {
		maxSize = 1000
	}
	
	return &CachedUserAgentParser{
		parser:  parser,
		cache:   make(map[string]UserAgentInfo),
		maxSize: maxSize,
	}
}

// Parse parses a user agent string with caching
func (p *CachedUserAgentParser) Parse(userAgent string) UserAgentInfo {
	if info, exists := p.cache[userAgent]; exists {
		return info
	}

	// If cache is full, clear it (simple eviction strategy)
	if len(p.cache) >= p.maxSize {
		p.cache = make(map[string]UserAgentInfo)
	}

	info := p.parser.Parse(userAgent)
	p.cache[userAgent] = info
	return info
}

// GetCacheStats returns cache statistics
func (p *CachedUserAgentParser) GetCacheStats() (size int, maxSize int) {
	return len(p.cache), p.maxSize
}

// ClearCache clears the parser cache
func (p *CachedUserAgentParser) ClearCache() {
	p.cache = make(map[string]UserAgentInfo)
}