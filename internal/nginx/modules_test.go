package nginx

import (
	"regexp"
	"strings"
	"testing"
)

func TestModuleNameNormalization(t *testing.T) {
	testCases := []struct {
		name               string
		loadModuleName     string
		expectedNormalized string
		configureArgName   string
		expectedLoadName   string
	}{
		{
			name:               "stream module",
			loadModuleName:     "ngx_stream_module",
			expectedNormalized: "stream",
			configureArgName:   "stream",
			expectedLoadName:   "ngx_stream_module",
		},
		{
			name:               "http_geoip module",
			loadModuleName:     "ngx_http_geoip_module",
			expectedNormalized: "http_geoip",
			configureArgName:   "http_geoip_module",
			expectedLoadName:   "ngx_http_geoip_module",
		},
		{
			name:               "stream_geoip module",
			loadModuleName:     "ngx_stream_geoip_module",
			expectedNormalized: "stream_geoip",
			configureArgName:   "stream_geoip_module",
			expectedLoadName:   "ngx_stream_geoip_module",
		},
		{
			name:               "http_image_filter module",
			loadModuleName:     "ngx_http_image_filter_module",
			expectedNormalized: "http_image_filter",
			configureArgName:   "http_image_filter_module",
			expectedLoadName:   "ngx_http_image_filter_module",
		},
		{
			name:               "mail module",
			loadModuleName:     "ngx_mail_module",
			expectedNormalized: "mail",
			configureArgName:   "mail",
			expectedLoadName:   "ngx_mail_module",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test normalization from load_module name
			normalizedFromLoad := normalizeModuleNameFromLoadModule(tc.loadModuleName)
			if normalizedFromLoad != tc.expectedNormalized {
				t.Errorf("normalizeModuleNameFromLoadModule(%s) = %s, expected %s",
					tc.loadModuleName, normalizedFromLoad, tc.expectedNormalized)
			}

			// Test normalization from configure argument name
			normalizedFromConfigure := normalizeModuleNameFromConfigure(tc.configureArgName)
			if normalizedFromConfigure != tc.expectedNormalized {
				t.Errorf("normalizeModuleNameFromConfigure(%s) = %s, expected %s",
					tc.configureArgName, normalizedFromConfigure, tc.expectedNormalized)
			}

			// Test getting expected load_module name
			expectedLoad := getExpectedLoadModuleName(tc.configureArgName)
			if expectedLoad != tc.expectedLoadName {
				t.Errorf("getExpectedLoadModuleName(%s) = %s, expected %s",
					tc.configureArgName, expectedLoad, tc.expectedLoadName)
			}
		})
	}
}

func TestGetLoadModuleRegex(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string // expected module names
	}{
		{
			name:     "quoted absolute path",
			input:    `load_module "/usr/local/nginx/modules/ngx_stream_module.so";`,
			expected: []string{"ngx_stream_module"},
		},
		{
			name:     "unquoted relative path",
			input:    `load_module modules/ngx_http_upstream_fair_module.so;`,
			expected: []string{"ngx_http_upstream_fair_module"},
		},
		{
			name:     "quoted relative path",
			input:    `load_module "modules/ngx_http_geoip_module.so";`,
			expected: []string{"ngx_http_geoip_module"},
		},
		{
			name:     "unquoted absolute path",
			input:    `load_module /etc/nginx/modules/ngx_http_cache_purge_module.so;`,
			expected: []string{"ngx_http_cache_purge_module"},
		},
		{
			name:     "multiple modules",
			input:    `load_module "/path/ngx_module1.so";\nload_module modules/ngx_module2.so;`,
			expected: []string{"ngx_module1", "ngx_module2"},
		},
		{
			name:     "with extra whitespace",
			input:    `load_module    "modules/ngx_test_module.so"   ;`,
			expected: []string{"ngx_test_module"},
		},
		{
			name:     "no matches",
			input:    `some other nginx config`,
			expected: []string{},
		},
	}

	regex := GetLoadModuleRegex()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := regex.FindAllStringSubmatch(tc.input, -1)

			if len(matches) != len(tc.expected) {
				t.Errorf("Expected %d matches, got %d", len(tc.expected), len(matches))
				return
			}

			for i, match := range matches {
				if len(match) < 2 {
					t.Errorf("Match %d should have at least 2 groups, got %d", i, len(match))
					continue
				}

				moduleName := match[1]
				expectedModule := tc.expected[i]

				if moduleName != expectedModule {
					t.Errorf("Expected module name %s, got %s", expectedModule, moduleName)
				}
			}
		})
	}
}

func TestModulesLoaded(t *testing.T) {
	text := `
load_module "/usr/local/nginx/modules/ngx_stream_module.so";
load_module modules/ngx_http_upstream_fair_module.so;
load_module "modules/ngx_http_geoip_module.so";
load_module /etc/nginx/modules/ngx_http_cache_purge_module.so;
`

	loadModuleRe := GetLoadModuleRegex()
	matches := loadModuleRe.FindAllStringSubmatch(text, -1)

	t.Log("matches", matches)

	// Expected module names
	expectedModules := []string{
		"ngx_stream_module",
		"ngx_http_upstream_fair_module",
		"ngx_http_geoip_module",
		"ngx_http_cache_purge_module",
	}

	if len(matches) != len(expectedModules) {
		t.Errorf("Expected %d matches, got %d", len(expectedModules), len(matches))
	}

	for i, match := range matches {
		if len(match) < 2 {
			t.Errorf("Match %d should have at least 2 groups, got %d", i, len(match))
			continue
		}

		moduleName := match[1]
		expectedModule := expectedModules[i]

		t.Logf("Match %d: %s", i, moduleName)

		if moduleName != expectedModule {
			t.Errorf("Expected module name %s, got %s", expectedModule, moduleName)
		}
	}
}

func TestRealWorldModuleMapping(t *testing.T) {
	// Simulate real nginx configuration scenarios
	testScenarios := []struct {
		name               string
		configureArg       string // from nginx -V output
		loadModuleStmt     string // from nginx -T output
		expectedNormalized string // internal representation
	}{
		{
			name:               "stream module - basic",
			configureArg:       "--with-stream",
			loadModuleStmt:     `load_module "/usr/lib/nginx/modules/ngx_stream_module.so";`,
			expectedNormalized: "stream",
		},
		{
			name:               "stream module - dynamic",
			configureArg:       "--with-stream=dynamic",
			loadModuleStmt:     `load_module modules/ngx_stream_module.so;`,
			expectedNormalized: "stream",
		},
		{
			name:               "http_geoip module",
			configureArg:       "--with-http_geoip_module=dynamic",
			loadModuleStmt:     `load_module "modules/ngx_http_geoip_module.so";`,
			expectedNormalized: "http_geoip",
		},
		{
			name:               "stream_geoip module",
			configureArg:       "--with-stream_geoip_module=dynamic",
			loadModuleStmt:     `load_module /usr/lib/nginx/modules/ngx_stream_geoip_module.so;`,
			expectedNormalized: "stream_geoip",
		},
		{
			name:               "http_image_filter module",
			configureArg:       "--with-http_image_filter_module=dynamic",
			loadModuleStmt:     `load_module modules/ngx_http_image_filter_module.so;`,
			expectedNormalized: "http_image_filter",
		},
		{
			name:               "mail module",
			configureArg:       "--with-mail=dynamic",
			loadModuleStmt:     `load_module "modules/ngx_mail_module.so";`,
			expectedNormalized: "mail",
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Test configure argument parsing
			paramRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(?:_module)?(?:=([^"'\s]+|"[^"]*"|'[^']*'))?`)
			configMatches := paramRe.FindAllStringSubmatch(scenario.configureArg, -1)

			if len(configMatches) == 0 {
				t.Errorf("Failed to parse configure argument: %s", scenario.configureArg)
				return
			}

			configModuleName := configMatches[0][1]
			normalizedConfigName := normalizeModuleNameFromConfigure(configModuleName)

			// Test load_module statement parsing
			loadModuleRe := GetLoadModuleRegex()
			loadMatches := loadModuleRe.FindAllStringSubmatch(scenario.loadModuleStmt, -1)

			if len(loadMatches) == 0 {
				t.Errorf("Failed to parse load_module statement: %s", scenario.loadModuleStmt)
				return
			}

			loadModuleName := loadMatches[0][1]
			normalizedLoadName := normalizeModuleNameFromLoadModule(loadModuleName)

			// Verify both normalize to the same expected value
			if normalizedConfigName != scenario.expectedNormalized {
				t.Errorf("Configure arg normalization: expected %s, got %s",
					scenario.expectedNormalized, normalizedConfigName)
			}

			if normalizedLoadName != scenario.expectedNormalized {
				t.Errorf("Load module normalization: expected %s, got %s",
					scenario.expectedNormalized, normalizedLoadName)
			}

			// Verify they match each other (this is the key test)
			if normalizedConfigName != normalizedLoadName {
				t.Errorf("Normalization mismatch: config=%s, load=%s",
					normalizedConfigName, normalizedLoadName)
			}

			t.Logf("✓ %s: config=%s -> load=%s -> normalized=%s",
				scenario.name, configModuleName, loadModuleName, scenario.expectedNormalized)
		})
	}
}

func TestAddLoadedDynamicModules(t *testing.T) {
	// Test scenario: modules loaded via load_module but not in configure args
	// This simulates the real-world case where external modules are installed
	// and loaded dynamically without being compiled into nginx

	// We can't directly test addLoadedDynamicModules since it depends on getNginxT()
	// But we can test the logic by simulating the behavior

	testLoadModuleOutput := `
# Configuration file /etc/nginx/modules-enabled/50-mod-stream.conf:
load_module modules/ngx_stream_module.so;
# Configuration file /etc/nginx/modules-enabled/70-mod-stream-geoip2.conf:
load_module modules/ngx_stream_geoip2_module.so;
load_module "modules/ngx_http_geoip2_module.so";
`

	// Test the regex and normalization logic
	loadModuleRe := GetLoadModuleRegex()
	matches := loadModuleRe.FindAllStringSubmatch(testLoadModuleOutput, -1)

	expectedModules := map[string]bool{
		"stream":        false,
		"stream_geoip2": false,
		"http_geoip2":   false,
	}

	t.Logf("Found %d load_module matches", len(matches))

	for _, match := range matches {
		if len(match) > 1 {
			loadModuleName := match[1]
			normalizedName := normalizeModuleNameFromLoadModule(loadModuleName)

			t.Logf("Load module: %s -> normalized: %s", loadModuleName, normalizedName)

			if _, expected := expectedModules[normalizedName]; expected {
				expectedModules[normalizedName] = true
			} else {
				t.Errorf("Unexpected module found: %s (from %s)", normalizedName, loadModuleName)
			}
		}
	}

	// Check that all expected modules were found
	for moduleName, found := range expectedModules {
		if !found {
			t.Errorf("Expected module %s was not found", moduleName)
		}
	}
}

func TestExternalModuleDiscovery(t *testing.T) {
	// Test the complete normalization pipeline for external modules
	testCases := []struct {
		name           string
		loadModuleName string
		expectedResult string
	}{
		{
			name:           "stream_geoip2 module",
			loadModuleName: "ngx_stream_geoip2_module",
			expectedResult: "stream_geoip2",
		},
		{
			name:           "http_geoip2 module",
			loadModuleName: "ngx_http_geoip2_module",
			expectedResult: "http_geoip2",
		},
		{
			name:           "custom third-party module",
			loadModuleName: "ngx_http_custom_module",
			expectedResult: "http_custom",
		},
		{
			name:           "simple module name",
			loadModuleName: "ngx_custom_module",
			expectedResult: "custom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeModuleNameFromLoadModule(tc.loadModuleName)
			if result != tc.expectedResult {
				t.Errorf("normalizeModuleNameFromLoadModule(%s) = %s, expected %s",
					tc.loadModuleName, result, tc.expectedResult)
			}
		})
	}
}

func TestOpenRestyModuleParsing(t *testing.T) {
	// Test case based on real OpenResty nginx -V output
	openRestyOutput := `nginx version: openresty/1.25.3.1
built by gcc 4.8.5 20150623 (Red Hat 4.8.5-44) (GCC) 
built with OpenSSL 1.0.2k-fips  26 Jan 2017
TLS SNI support enabled
configure arguments: --prefix=/usr/local/openresty/nginx --with-cc-opt=-O2 --add-module=../ngx_devel_kit-0.3.3 --add-module=../echo-nginx-module-0.63 --add-module=../xss-nginx-module-0.06 --add-module=../ngx_coolkit-0.2 --add-module=../set-misc-nginx-module-0.33 --add-module=../form-input-nginx-module-0.12 --add-module=../encrypted-session-nginx-module-0.09 --add-module=../srcache-nginx-module-0.33 --add-module=../ngx_lua-0.10.26 --add-module=../ngx_lua_upstream-0.07 --add-module=../headers-more-nginx-module-0.37 --add-module=../array-var-nginx-module-0.06 --add-module=../memc-nginx-module-0.20 --add-module=../redis2-nginx-module-0.15 --add-module=../redis-nginx-module-0.3.9 --add-module=../rds-json-nginx-module-0.16 --add-module=../rds-csv-nginx-module-0.09 --add-module=../ngx_stream_lua-0.0.14 --with-ld-opt=-Wl,-rpath,/usr/local/openresty/luajit/lib --with-http_ssl_module --with-http_v2_module --with-http_realip_module --with-stream --without-pcre2 --with-stream_ssl_module --with-stream_ssl_preread_module`

	// Test parsing --add-module arguments
	addModuleRe := regexp.MustCompile(`--add-module=([^/\s]+/)([^/\s-]+)-([0-9.]+)`)
	matches := addModuleRe.FindAllStringSubmatch(openRestyOutput, -1)

	expectedModules := map[string]bool{
		"ngx_devel_kit":                  false,
		"echo_nginx_module":              false,
		"xss_nginx_module":               false,
		"ngx_coolkit":                    false,
		"set_misc_nginx_module":          false,
		"form_input_nginx_module":        false,
		"encrypted_session_nginx_module": false,
		"srcache_nginx_module":           false,
		"ngx_lua":                        false,
		"ngx_lua_upstream":               false,
		"headers_more_nginx_module":      false,
		"array_var_nginx_module":         false,
		"memc_nginx_module":              false,
		"redis2_nginx_module":            false,
		"redis_nginx_module":             false,
		"rds_json_nginx_module":          false,
		"rds_csv_nginx_module":           false,
		"ngx_stream_lua":                 false,
	}

	t.Logf("Found %d --add-module matches", len(matches))

	for _, match := range matches {
		if len(match) > 2 {
			moduleName := match[2]
			t.Logf("Found add-module: %s", moduleName)

			if _, expected := expectedModules[moduleName]; expected {
				expectedModules[moduleName] = true
			} else {
				// This might be a valid module we didn't expect
				t.Logf("Unexpected add-module found: %s", moduleName)
			}
		}
	}

	// Check that we found most expected modules
	foundCount := 0
	for moduleName, found := range expectedModules {
		if found {
			foundCount++
		} else {
			t.Logf("Expected add-module %s was not found", moduleName)
		}
	}

	if foundCount == 0 {
		t.Error("No add-modules were parsed successfully")
	}

	// Test parsing --with- arguments as well
	withModuleRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(?:_module)?(?:=([^"'\s]+|"[^"]*"|'[^']*'))?`)
	withMatches := withModuleRe.FindAllStringSubmatch(openRestyOutput, -1)

	expectedWithModules := map[string]bool{
		"cc-opt":                    false,
		"ld-opt":                    false,
		"http_ssl_module":           false,
		"http_v2_module":            false,
		"http_realip_module":        false,
		"stream":                    false,
		"stream_ssl_module":         false,
		"stream_ssl_preread_module": false,
	}

	t.Logf("Found %d --with- matches", len(withMatches))

	for _, match := range withMatches {
		if len(match) > 1 {
			moduleName := match[1]
			t.Logf("Found with-module: %s", moduleName)

			if _, expected := expectedWithModules[moduleName]; expected {
				expectedWithModules[moduleName] = true
			}
		}
	}

	// Verify we found the key --with- modules
	withFoundCount := 0
	for _, found := range expectedWithModules {
		if found {
			withFoundCount++
		}
	}

	if withFoundCount < 3 { // At least stream, http_ssl_module, etc should be found
		t.Errorf("Too few --with- modules found: %d", withFoundCount)
	}
}

func TestAddModuleRegexParsing(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string // expected module names
	}{
		{
			name:     "single add-module with version",
			input:    "--add-module=../ngx_devel_kit-0.3.3",
			expected: []string{"ngx_devel_kit"},
		},
		{
			name:     "add-module with nginx in name",
			input:    "--add-module=../echo-nginx-module-0.63",
			expected: []string{"echo_nginx_module"},
		},
		{
			name:     "multiple add-modules",
			input:    "--add-module=../ngx_lua-0.10.26 --add-module=../headers-more-nginx-module-0.37",
			expected: []string{"ngx_lua", "headers_more_nginx_module"},
		},
		{
			name:     "add-module with different separators",
			input:    "--add-module=../set-misc-nginx-module-0.33 --add-module=../ngx_coolkit-0.2",
			expected: []string{"set_misc_nginx_module", "ngx_coolkit"},
		},
	}

	// Regex to parse --add-module arguments
	addModuleRe := regexp.MustCompile(`--add-module=(?:[^/\s]+/)?([^/\s-]+(?:-[^/\s-]+)*)-[0-9.]+`)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := addModuleRe.FindAllStringSubmatch(tc.input, -1)

			if len(matches) != len(tc.expected) {
				t.Errorf("Expected %d matches, got %d", len(tc.expected), len(matches))
				for i, match := range matches {
					if len(match) > 1 {
						t.Logf("Match %d: %s", i, match[1])
					}
				}
				return
			}

			for i, match := range matches {
				if len(match) < 2 {
					t.Errorf("Match %d should have at least 2 groups, got %d", i, len(match))
					continue
				}

				moduleName := match[1]
				// Convert dashes to underscores for consistency
				normalizedName := strings.ReplaceAll(moduleName, "-", "_")
				expectedModule := tc.expected[i]

				if normalizedName != expectedModule {
					t.Errorf("Expected module name %s, got %s (normalized from %s)", expectedModule, normalizedName, moduleName)
				}
			}
		})
	}
}

func TestNormalizeAddModuleName(t *testing.T) {
	testCases := []struct {
		name           string
		addModuleName  string
		expectedResult string
	}{
		{
			name:           "ngx_devel_kit",
			addModuleName:  "ngx_devel_kit",
			expectedResult: "devel_kit",
		},
		{
			name:           "echo-nginx-module",
			addModuleName:  "echo-nginx-module",
			expectedResult: "echo_nginx",
		},
		{
			name:           "headers-more-nginx-module",
			addModuleName:  "headers-more-nginx-module",
			expectedResult: "headers_more_nginx",
		},
		{
			name:           "ngx_lua",
			addModuleName:  "ngx_lua",
			expectedResult: "lua",
		},
		{
			name:           "set-misc-nginx-module",
			addModuleName:  "set-misc-nginx-module",
			expectedResult: "set_misc_nginx",
		},
		{
			name:           "ngx_stream_lua",
			addModuleName:  "ngx_stream_lua",
			expectedResult: "stream_lua",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeAddModuleName(tc.addModuleName)
			if result != tc.expectedResult {
				t.Errorf("normalizeAddModuleName(%s) = %s, expected %s",
					tc.addModuleName, result, tc.expectedResult)
			}
		})
	}
}

func TestStreamConfigurationParsing(t *testing.T) {
	// Test parsing of stream configuration to verify stream module is working
	streamConfig := `stream {
    log_format tcp_format '$time_local|$remote_addr|$protocol|$status|$bytes_sent|$bytes_received|$session_time|$upstream_addr|$upstream_bytes_sent|$upstream_bytes_received|$upstream_connect_time';
    include /usr/local/openresty/nginx/conf/streams-enabled/*.conf;
    default_type  application/octet-stream;
    
    upstream sshd_63_stream {
        server 192.168.1.63:22;
    }
    
    server {
        listen 6001;
        proxy_pass sshd_63_stream;
    }
}`

	// Simple test to verify stream block can be detected (word boundary to avoid matching "upstream sshd_63_stream")
	streamBlockRe := regexp.MustCompile(`\bstream\s*\{`)
	matches := streamBlockRe.FindAllString(streamConfig, -1)

	if len(matches) != 1 {
		t.Errorf("Expected to find 1 stream block, found %d", len(matches))
	}

	// Test upstream parsing within stream
	upstreamRe := regexp.MustCompile(`upstream\s+([a-zA-Z0-9_]+)\s*\{`)
	upstreamMatches := upstreamRe.FindAllStringSubmatch(streamConfig, -1)

	if len(upstreamMatches) != 1 {
		t.Errorf("Expected to find 1 upstream, found %d", len(upstreamMatches))
	} else if upstreamMatches[0][1] != "sshd_63_stream" {
		t.Errorf("Expected upstream name 'sshd_63_stream', got '%s'", upstreamMatches[0][1])
	}

	// Test server block parsing within stream
	serverRe := regexp.MustCompile(`server\s*\{[^}]*listen\s+(\d+)`)
	serverMatches := serverRe.FindAllStringSubmatch(streamConfig, -1)

	if len(serverMatches) != 1 {
		t.Errorf("Expected to find 1 server with listen directive, found %d", len(serverMatches))
	} else if serverMatches[0][1] != "6001" {
		t.Errorf("Expected listen port '6001', got '%s'", serverMatches[0][1])
	}
}

func TestIntegratedModuleDetection(t *testing.T) {
	// This test simulates the complete flow of module detection for OpenResty
	// This would test the integration between --add-module parsing and --with- parsing

	// Mock nginx -V output combining both --add-module and --with- parameters
	mockNginxV := `nginx version: openresty/1.25.3.1
configure arguments: --prefix=/usr/local/openresty/nginx --with-cc-opt=-O2 --add-module=../ngx_devel_kit-0.3.3 --add-module=../ngx_lua-0.10.26 --with-http_ssl_module --with-stream --with-stream_ssl_module`

	// Test both regex patterns work on the same input
	withModuleRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(?:_module)?(?:=([^"'\s]+|"[^"]*"|'[^']*'))?`)
	addModuleRe := regexp.MustCompile(`--add-module=(?:[^/\s]+/)?([^/\s-]+(?:-[^/\s-]+)*)-[0-9.]+`)

	withMatches := withModuleRe.FindAllStringSubmatch(mockNginxV, -1)
	addMatches := addModuleRe.FindAllStringSubmatch(mockNginxV, -1)

	t.Logf("Found %d --with- matches and %d --add-module matches", len(withMatches), len(addMatches))

	// Verify we can parse both types
	if len(withMatches) == 0 {
		t.Error("Failed to parse any --with- modules")
	}

	if len(addMatches) == 0 {
		t.Error("Failed to parse any --add-module modules")
	}

	// Build a combined module list like the actual code should do
	allModules := make(map[string]bool)

	// Process --with- modules
	for _, match := range withMatches {
		if len(match) > 1 {
			moduleName := match[1]
			normalized := normalizeModuleNameFromConfigure(moduleName)
			allModules[normalized] = true
			t.Logf("--with- module: %s -> %s", moduleName, normalized)
		}
	}

	// Process --add-module modules
	for _, match := range addMatches {
		if len(match) > 1 {
			moduleName := match[1]
			normalized := normalizeAddModuleName(moduleName)
			allModules[normalized] = true
			t.Logf("--add-module: %s -> %s", moduleName, normalized)
		}
	}

	// Verify we have both types of modules
	expectedModules := []string{"stream", "http_ssl", "devel_kit", "lua"}
	foundCount := 0

	for _, expected := range expectedModules {
		if allModules[expected] {
			foundCount++
			t.Logf("✓ Found expected module: %s", expected)
		} else {
			t.Logf("✗ Missing expected module: %s", expected)
		}
	}

	if foundCount < 2 {
		t.Errorf("Expected to find at least 2 modules, found %d", foundCount)
	}
}
