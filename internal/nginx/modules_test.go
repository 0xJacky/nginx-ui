package nginx

import (
	"regexp"
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

func TestGetModuleMapping(t *testing.T) {
	// This test verifies that GetModuleMapping function works without errors
	// Since it depends on nginx being available, we'll just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetModuleMapping panicked: %v", r)
		}
	}()

	mapping := GetModuleMapping()

	// The mapping should be a valid map (could be empty if nginx is not available)
	if mapping == nil {
		t.Error("GetModuleMapping returned nil")
	}

	t.Logf("GetModuleMapping returned %d entries", len(mapping))

	// If there are entries, verify they have the expected structure
	for moduleName, moduleInfo := range mapping {
		if moduleInfo == nil {
			t.Errorf("Module %s has nil info", moduleName)
			continue
		}

		requiredFields := []string{"normalized", "expected_load_module", "dynamic", "loaded", "params"}
		for _, field := range requiredFields {
			if _, exists := moduleInfo[field]; !exists {
				t.Errorf("Module %s missing field %s", moduleName, field)
			}
		}
	}
}
