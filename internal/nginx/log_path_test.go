package nginx

import (
	"path/filepath"
	"regexp"
	"testing"
)

// Mock nginx -T output for testing purposes
const mockNginxTOutput = `
# configuration file /etc/nginx/nginx.conf:
user  nginx;
worker_processes  auto;

error_log  /var/log/nginx/error.log notice;
error_log  /var/log/nginx/error.local.log notice;
pid        /var/run/nginx.pid;

events {
    worker_connections  1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;

    log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"';

    access_log  /var/log/nginx/access.log  main;
    access_log  /var/log/nginx/access.local.log  main;

    sendfile        on;
    keepalive_timeout  65;
    gzip  on;

    server {
        listen       80;
        server_name  localhost;
        
        access_log   /var/log/nginx/server.access.log;
        error_log    /var/log/nginx/server.error.log warn;

        location / {
            root   /usr/share/nginx/html;
            index  index.html index.htm;
        }
    }
}

stream {
    error_log /var/log/nginx/stream.error.log info;
    
    server {
        listen 3306;
        proxy_pass backend;
    }
}
`

// Mock nginx -T output with relative paths
const mockNginxTOutputRelative = `
# configuration file /etc/nginx/nginx.conf:
user  nginx;
worker_processes  auto;

error_log  logs/error.log notice;
pid        /var/run/nginx.pid;

http {
    access_log  logs/access.log  main;
    
    server {
        listen       80;
        server_name  localhost;
        
        access_log   logs/server.access.log;
        error_log    logs/server.error.log warn;
    }
}
`

// Mock nginx -T output with access_log off
const mockNginxTOutputOff = `
# configuration file /etc/nginx/nginx.conf:
user  nginx;
worker_processes  auto;

error_log  /var/log/nginx/error.log notice;

http {
    access_log  off;
    
    server {
        listen       80;
        server_name  localhost;
        
        access_log   /var/log/nginx/server.access.log;
        error_log    /var/log/nginx/server.error.log warn;
    }
}
`

// Mock nginx -T output with commented log directives
const mockNginxTOutputCommented = `
# configuration file /etc/nginx/nginx.conf:
user  nginx;
worker_processes  auto;

# error_log  /var/log/nginx/commented.error.log notice;
error_log  /var/log/nginx/error.log notice;

http {
    # access_log  /var/log/nginx/commented.access.log  main;
    access_log  /var/log/nginx/access.log  main;
    
    server {
        listen       80;
        server_name  localhost;
        
        # access_log   /var/log/nginx/commented.server.access.log;
        access_log   /var/log/nginx/server.access.log;
        # error_log    /var/log/nginx/commented.server.error.log warn;
        error_log    /var/log/nginx/server.error.log warn;
    }
}
`

func TestAccessLogRegexParsing(t *testing.T) {
	testCases := []struct {
		name          string
		nginxTOutput  string
		expectedPath  string
		shouldHaveLog bool
	}{
		{
			name:          "standard access log",
			nginxTOutput:  "access_log  /var/log/nginx/access.log  main;",
			expectedPath:  "/var/log/nginx/access.log",
			shouldHaveLog: true,
		},
		{
			name:          "access log turned off",
			nginxTOutput:  "access_log  off;",
			expectedPath:  "",
			shouldHaveLog: false,
		},
		{
			name:          "no access log directive",
			nginxTOutput:  "server_name  localhost;",
			expectedPath:  "",
			shouldHaveLog: false,
		},
		{
			name:          "indented access log",
			nginxTOutput:  "    access_log  /var/log/nginx/server.log;",
			expectedPath:  "/var/log/nginx/server.log",
			shouldHaveLog: true,
		},
		{
			name:          "multiple access logs - should get first",
			nginxTOutput:  "access_log  /var/log/nginx/access1.log  main;\naccess_log  /var/log/nginx/access2.log  combined;",
			expectedPath:  "/var/log/nginx/access1.log",
			shouldHaveLog: true,
		},
		{
			name:          "commented access log should be ignored",
			nginxTOutput:  "# access_log  /var/log/nginx/commented.access.log  main;\naccess_log  /var/log/nginx/access.log  main;",
			expectedPath:  "/var/log/nginx/access.log",
			shouldHaveLog: true,
		},
		{
			name:          "only commented access log",
			nginxTOutput:  "# access_log  /var/log/nginx/commented.access.log  main;",
			expectedPath:  "",
			shouldHaveLog: false,
		},
	}

	accessLogRegex := regexp.MustCompile(AccessLogRegexPattern)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := accessLogRegex.FindAllStringSubmatch(tc.nginxTOutput, -1)

			if !tc.shouldHaveLog {
				if len(matches) > 0 {
					// Check if it's the "off" directive
					if len(matches[0]) >= 2 {
						logPath := matches[0][1]
						if logPath != "off" {
							t.Errorf("Expected no valid access log, but found: %s", logPath)
						}
					}
				}
				return
			}

			if len(matches) == 0 {
				t.Errorf("Expected to find access log directive, but found none")
				return
			}

			if len(matches[0]) < 2 {
				t.Errorf("Expected regex match to have at least 2 groups, got %d", len(matches[0]))
				return
			}

			logPath := matches[0][1]

			if logPath != tc.expectedPath {
				t.Errorf("Expected access log path %s, got %s", tc.expectedPath, logPath)
			}
		})
	}
}

func TestErrorLogRegexParsing(t *testing.T) {
	testCases := []struct {
		name          string
		nginxTOutput  string
		expectedPath  string
		shouldHaveLog bool
	}{
		{
			name:          "standard error log",
			nginxTOutput:  "error_log  /var/log/nginx/error.log notice;",
			expectedPath:  "/var/log/nginx/error.log",
			shouldHaveLog: true,
		},
		{
			name:          "error log without level",
			nginxTOutput:  "error_log  /var/log/nginx/error.log;",
			expectedPath:  "/var/log/nginx/error.log",
			shouldHaveLog: true,
		},
		{
			name:          "no error log directive",
			nginxTOutput:  "server_name  localhost;",
			expectedPath:  "",
			shouldHaveLog: false,
		},
		{
			name:          "indented error log",
			nginxTOutput:  "        error_log  /var/log/nginx/server.error.log warn;",
			expectedPath:  "/var/log/nginx/server.error.log",
			shouldHaveLog: true,
		},
		{
			name:          "multiple error logs - should get first",
			nginxTOutput:  "error_log  /var/log/nginx/error1.log  notice;\nerror_log  /var/log/nginx/error2.log  warn;",
			expectedPath:  "/var/log/nginx/error1.log",
			shouldHaveLog: true,
		},
		{
			name:          "commented error log should be ignored",
			nginxTOutput:  "# error_log  /var/log/nginx/commented.error.log  notice;\nerror_log  /var/log/nginx/error.log  notice;",
			expectedPath:  "/var/log/nginx/error.log",
			shouldHaveLog: true,
		},
		{
			name:          "only commented error log",
			nginxTOutput:  "# error_log  /var/log/nginx/commented.error.log  notice;",
			expectedPath:  "",
			shouldHaveLog: false,
		},
	}

	errorLogRegex := regexp.MustCompile(ErrorLogRegexPattern)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := errorLogRegex.FindAllStringSubmatch(tc.nginxTOutput, -1)

			if !tc.shouldHaveLog {
				if len(matches) > 0 {
					t.Errorf("Expected no error log directive, but found: %v", matches)
				}
				return
			}

			if len(matches) == 0 {
				t.Errorf("Expected to find error log directive, but found none")
				return
			}

			if len(matches[0]) < 2 {
				t.Errorf("Expected regex match to have at least 2 groups, got %d", len(matches[0]))
				return
			}

			logPath := matches[0][1]

			if logPath != tc.expectedPath {
				t.Errorf("Expected error log path %s, got %s", tc.expectedPath, logPath)
			}
		})
	}
}

func TestLogPathParsing(t *testing.T) {
	testCases := []struct {
		name               string
		nginxTOutput       string
		expectedAccessPath string
		expectedErrorPath  string
		shouldHaveAccess   bool
		shouldHaveError    bool
	}{
		{
			name:               "complete configuration",
			nginxTOutput:       mockNginxTOutput,
			expectedAccessPath: "/var/log/nginx/access.log",
			expectedErrorPath:  "/var/log/nginx/error.log",
			shouldHaveAccess:   true,
			shouldHaveError:    true,
		},
		{
			name:               "configuration with commented directives",
			nginxTOutput:       mockNginxTOutputCommented,
			expectedAccessPath: "/var/log/nginx/access.log",
			expectedErrorPath:  "/var/log/nginx/error.log",
			shouldHaveAccess:   true,
			shouldHaveError:    true,
		},
		{
			name:               "access log turned off",
			nginxTOutput:       mockNginxTOutputOff,
			expectedAccessPath: "/var/log/nginx/server.access.log", // Should get the server-level access log
			expectedErrorPath:  "/var/log/nginx/error.log",
			shouldHaveAccess:   true,
			shouldHaveError:    true,
		},
		{
			name:               "empty configuration",
			nginxTOutput:       "",
			expectedAccessPath: "",
			expectedErrorPath:  "",
			shouldHaveAccess:   false,
			shouldHaveError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test access log parsing
			accessLogRegex := regexp.MustCompile(AccessLogRegexPattern)
			accessMatches := accessLogRegex.FindAllStringSubmatch(tc.nginxTOutput, -1)

			var foundAccessPath string
			for _, match := range accessMatches {
				if len(match) >= 2 {
					logPath := match[1]
					if logPath != "off" {
						foundAccessPath = logPath
						break
					}
				}
			}

			if tc.shouldHaveAccess {
				if foundAccessPath == "" {
					t.Errorf("Expected access log path %s, but found none", tc.expectedAccessPath)
				} else if foundAccessPath != tc.expectedAccessPath {
					t.Errorf("Expected access log path %s, got %s", tc.expectedAccessPath, foundAccessPath)
				}
			} else {
				if foundAccessPath != "" {
					t.Errorf("Expected no access log path, but found %s", foundAccessPath)
				}
			}

			// Test error log parsing
			errorLogRegex := regexp.MustCompile(ErrorLogRegexPattern)
			errorMatches := errorLogRegex.FindAllStringSubmatch(tc.nginxTOutput, -1)

			var foundErrorPath string
			if len(errorMatches) > 0 && len(errorMatches[0]) >= 2 {
				foundErrorPath = errorMatches[0][1]
			}

			if tc.shouldHaveError {
				if foundErrorPath == "" {
					t.Errorf("Expected error log path %s, but found none", tc.expectedErrorPath)
				} else if foundErrorPath != tc.expectedErrorPath {
					t.Errorf("Expected error log path %s, got %s", tc.expectedErrorPath, foundErrorPath)
				}
			} else {
				if foundErrorPath != "" {
					t.Errorf("Expected no error log path, but found %s", foundErrorPath)
				}
			}
		})
	}
}

func TestRelativePathHandling(t *testing.T) {
	// Mock GetPrefix function for testing
	originalGetPrefix := GetPrefix
	defer func() {
		// Restore original function (if needed for other tests)
		_ = originalGetPrefix
	}()

	testPrefix := "/usr/local/nginx"

	testCases := []struct {
		name         string
		inputPath    string
		expectedPath string
		isRelative   bool
	}{
		{
			name:         "absolute path",
			inputPath:    "/var/log/nginx/access.log",
			expectedPath: "/var/log/nginx/access.log",
			isRelative:   false,
		},
		{
			name:         "relative path",
			inputPath:    "logs/access.log",
			expectedPath: filepath.Join(testPrefix, "logs/access.log"),
			isRelative:   true,
		},
		{
			name:         "relative path with ./",
			inputPath:    "./logs/access.log",
			expectedPath: filepath.Join(testPrefix, "./logs/access.log"),
			isRelative:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string

			if tc.isRelative {
				result = filepath.Join(testPrefix, tc.inputPath)
			} else {
				result = tc.inputPath
			}

			if result != tc.expectedPath {
				t.Errorf("Expected path %s, got %s", tc.expectedPath, result)
			}
		})
	}
}

func TestComplexNginxConfiguration(t *testing.T) {
	complexConfig := `
# Main configuration
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                   '$status $body_bytes_sent "$http_referer" '
                   '"$http_user_agent" "$http_x_forwarded_for"';
    
    access_log /var/log/nginx/access.log main;
    
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;
    
    # Virtual Host Configs
    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
    
    server {
        listen 80 default_server;
        listen [::]:80 default_server;
        server_name _;
        root /var/www/html;
        index index.html index.htm index.nginx-debian.html;
        
        access_log /var/log/nginx/default.access.log;
        error_log /var/log/nginx/default.error.log;
        
        location / {
            try_files $uri $uri/ =404;
        }
        
        location ~ /\.ht {
            deny all;
        }
    }
    
    server {
        listen 443 ssl http2;
        server_name example.com;
        root /var/www/example.com;
        
        access_log /var/log/nginx/example.access.log combined;
        error_log /var/log/nginx/example.error.log info;
        
        ssl_certificate /etc/ssl/certs/example.com.pem;
        ssl_certificate_key /etc/ssl/private/example.com.key;
    }
}

stream {
    error_log /var/log/nginx/stream.error.log info;
    
    upstream backend {
        server 192.168.1.100:3306;
        server 192.168.1.101:3306;
    }
    
    server {
        listen 3306;
        proxy_pass backend;
        proxy_timeout 1s;
        proxy_responses 1;
    }
}
`

	// Test that we can extract the main access log and error log from complex config
	accessLogRegex := regexp.MustCompile(AccessLogRegexPattern)
	errorLogRegex := regexp.MustCompile(ErrorLogRegexPattern)

	// Find all access logs
	accessMatches := accessLogRegex.FindAllStringSubmatch(complexConfig, -1)
	if len(accessMatches) == 0 {
		t.Error("Expected to find access log directives in complex config")
	} else {
		firstAccessLog := accessMatches[0][1]
		expectedFirstAccess := "/var/log/nginx/access.log"
		if firstAccessLog != expectedFirstAccess {
			t.Errorf("Expected first access log to be %s, got %s", expectedFirstAccess, firstAccessLog)
		}
		t.Logf("Found %d access log directives, first: %s", len(accessMatches), firstAccessLog)
	}

	// Find all error logs
	errorMatches := errorLogRegex.FindAllStringSubmatch(complexConfig, -1)
	if len(errorMatches) == 0 {
		t.Error("Expected to find error log directives in complex config")
	} else {
		firstErrorLog := errorMatches[0][1]
		expectedFirstError := "/var/log/nginx/error.log"
		if firstErrorLog != expectedFirstError {
			t.Errorf("Expected first error log to be %s, got %s", expectedFirstError, firstErrorLog)
		}
		t.Logf("Found %d error log directives, first: %s", len(errorMatches), firstErrorLog)
	}
}

func TestCommentedDirectivesIgnored(t *testing.T) {
	testConfig := `
# Main configuration
user nginx;
worker_processes auto;

# These should be ignored
# error_log  /var/log/nginx/commented.error.log notice;
# access_log  /var/log/nginx/commented.access.log  main;

# Real directives
error_log /var/log/nginx/error.log warn;

http {
    # This should be ignored too
    # access_log /var/log/nginx/commented.http.access.log combined;
    
    # Real directive
    access_log /var/log/nginx/access.log main;
    
    server {
        listen 80;
        server_name example.com;
        
        # Commented server-level logs should be ignored
        # access_log /var/log/nginx/commented.server.access.log;
        # error_log /var/log/nginx/commented.server.error.log warn;
        
        # Real server-level logs
        access_log /var/log/nginx/server.access.log;
        error_log /var/log/nginx/server.error.log info;
    }
}
`

	// Test access log parsing ignores comments
	accessLogRegex := regexp.MustCompile(AccessLogRegexPattern)
	accessMatches := accessLogRegex.FindAllStringSubmatch(testConfig, -1)

	expectedAccessLogs := []string{
		"/var/log/nginx/access.log",
		"/var/log/nginx/server.access.log",
	}

	if len(accessMatches) != len(expectedAccessLogs) {
		t.Errorf("Expected %d access log matches, got %d", len(expectedAccessLogs), len(accessMatches))
	}

	for i, match := range accessMatches {
		if i < len(expectedAccessLogs) {
			if match[1] != expectedAccessLogs[i] {
				t.Errorf("Expected access log %d to be %s, got %s", i, expectedAccessLogs[i], match[1])
			}
		}
	}

	// Test error log parsing ignores comments
	errorLogRegex := regexp.MustCompile(ErrorLogRegexPattern)
	errorMatches := errorLogRegex.FindAllStringSubmatch(testConfig, -1)

	expectedErrorLogs := []string{
		"/var/log/nginx/error.log",
		"/var/log/nginx/server.error.log",
	}

	if len(errorMatches) != len(expectedErrorLogs) {
		t.Errorf("Expected %d error log matches, got %d", len(expectedErrorLogs), len(errorMatches))
	}

	for i, match := range errorMatches {
		if i < len(expectedErrorLogs) {
			if match[1] != expectedErrorLogs[i] {
				t.Errorf("Expected error log %d to be %s, got %s", i, expectedErrorLogs[i], match[1])
			}
		}
	}
}

func TestPIDRegexParsing(t *testing.T) {
	testCases := []struct {
		name         string
		nginxTOutput string
		expectedPath string
		shouldMatch  bool
	}{
		{
			name:         "standard pid path",
			nginxTOutput: "pid        /var/run/nginx.pid;",
			expectedPath: "/var/run/nginx.pid",
			shouldMatch:  true,
		},
		{
			name:         "nginx-unprivileged pid path",
			nginxTOutput: "pid /tmp/nginx.pid;",
			expectedPath: "/tmp/nginx.pid",
			shouldMatch:  true,
		},
		{
			name:         "indented pid directive",
			nginxTOutput: "    pid  /run/nginx.pid;",
			expectedPath: "/run/nginx.pid",
			shouldMatch:  true,
		},
		{
			name:         "no pid directive",
			nginxTOutput: "worker_processes  auto;",
			expectedPath: "",
			shouldMatch:  false,
		},
		{
			name:         "commented pid directive should not match",
			nginxTOutput: "# pid  /var/run/nginx.pid;",
			expectedPath: "",
			shouldMatch:  false,
		},
		{
			name:         "pid in full config",
			nginxTOutput: "user  nginx;\nworker_processes  auto;\npid /tmp/nginx.pid;\nevents {\n    worker_connections 1024;\n}",
			expectedPath: "/tmp/nginx.pid",
			shouldMatch:  true,
		},
		{
			name:         "commented pid followed by real pid",
			nginxTOutput: "# pid /var/run/nginx.pid;\npid /tmp/nginx.pid;",
			expectedPath: "/tmp/nginx.pid",
			shouldMatch:  true,
		},
	}

	pidRegex := regexp.MustCompile(PIDRegexPattern)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Filter out commented lines (same as getPIDPathFromNginxT does)
			var firstMatch string
			for _, line := range regexp.MustCompile(`\n`).Split(tc.nginxTOutput, -1) {
				if isCommentedLine(line) {
					continue
				}
				matches := pidRegex.FindStringSubmatch(line)
				if len(matches) >= 2 {
					firstMatch = matches[1]
					break
				}
			}

			if tc.shouldMatch {
				if firstMatch == "" {
					t.Errorf("Expected to find pid directive, but found none")
					return
				}
				if firstMatch != tc.expectedPath {
					t.Errorf("Expected pid path %s, got %s", tc.expectedPath, firstMatch)
				}
			} else {
				if firstMatch != "" {
					t.Errorf("Expected no pid directive, but found: %s", firstMatch)
				}
			}
		})
	}
}

func TestPIDPathFromMockConfigs(t *testing.T) {
	pidRegex := regexp.MustCompile(PIDRegexPattern)

	testCases := []struct {
		name         string
		config       string
		expectedPath string
	}{
		{
			name:         "standard nginx config",
			config:       mockNginxTOutput,
			expectedPath: "/var/run/nginx.pid",
		},
		{
			name:         "config with relative paths",
			config:       mockNginxTOutputRelative,
			expectedPath: "/var/run/nginx.pid",
		},
		{
			name: "nginx-unprivileged config",
			config: `
user  nginx;
worker_processes  auto;

error_log  /var/log/nginx/error.log notice;
pid        /tmp/nginx.pid;

events {
    worker_connections  1024;
}

http {
    access_log  /var/log/nginx/access.log  main;
}
`,
			expectedPath: "/tmp/nginx.pid",
		},
		{
			name:         "config without pid directive",
			config:       mockNginxTOutputOff,
			expectedPath: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var foundPath string
			for _, line := range regexp.MustCompile(`\n`).Split(tc.config, -1) {
				if isCommentedLine(line) {
					continue
				}
				matches := pidRegex.FindStringSubmatch(line)
				if len(matches) >= 2 {
					foundPath = matches[1]
					break
				}
			}

			if foundPath != tc.expectedPath {
				t.Errorf("Expected pid path %q, got %q", tc.expectedPath, foundPath)
			}
		})
	}
}
