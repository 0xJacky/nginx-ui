package demo

// Traffic vocabulary for the synthetic access log. Kept beside the geo tables
// so the shape of the demo's traffic is one reviewable file.

// requestPaths are weighted by repetition: the ones that appear more often show
// up more often in the log, which is what makes the "top paths" panel look like
// a real site rather than a uniform histogram.
var requestPaths = []string{
	"/", "/", "/", "/",
	"/api/v1/status", "/api/v1/status", "/api/v1/status",
	"/api/v1/users", "/api/v1/users",
	"/api/v1/orders",
	"/api/v1/products?page=1", "/api/v1/products?page=2",
	"/assets/index.a3f9c2.js", "/assets/index.a3f9c2.js",
	"/assets/vendor.7d1e8b.js",
	"/assets/style.4c2a9f.css",
	"/favicon.ico",
	"/docs/getting-started", "/docs/configuration",
	"/blog/2026/nginx-tuning",
	"/health",
	"/login", "/logout",
	"/robots.txt",
	"/.env",
	"/wp-admin/setup-config.php",
	"/admin/../../etc/passwd",
}

// methodsByPath keeps verbs plausible: the API write endpoints see POST, the
// static assets never do.
var writePaths = map[string]bool{
	"/api/v1/users":  true,
	"/api/v1/orders": true,
	"/login":         true,
	"/logout":        true,
}

// probePaths draw 404s and 403s, so the status breakdown is not all 2xx and
// the "errors" view has something to show.
var probePaths = map[string]bool{
	"/.env":                      true,
	"/wp-admin/setup-config.php": true,
	"/admin/../../etc/passwd":    true,
}

var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.4 Safari/605.1.15",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	"curl/8.9.1",
	"python-requests/2.32.3",
}

var referers = []string{
	"-", "-", "-",
	"https://www.google.com/",
	"https://github.com/0xJacky/nginx-ui",
	"https://news.ycombinator.com/",
	"https://demo.nginxui.com/",
}
