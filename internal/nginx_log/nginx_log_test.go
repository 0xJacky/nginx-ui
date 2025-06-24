package nginx_log

import (
	"testing"
)

// TestScanForLogDirectivesRemoval tests that removed log directives are properly cleaned up
func TestScanForLogDirectivesRemoval(t *testing.T) {
	// Clear cache before test
	ClearLogCache()

	configPath := "/etc/nginx/sites-available/test.conf"

	// First scan with two log directives
	content1 := []byte(`
server {
    listen 80;
    server_name example.com;
    
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;
}
`)

	err := scanForLogDirectives(configPath, content1)
	if err != nil {
		t.Fatalf("First scan failed: %v", err)
	}

	// Check that both logs are cached
	logs := GetAllLogPaths()
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs after first scan, got %d", len(logs))
	}

	// Verify the config file is tracked
	accessFound := false
	errorFound := false
	for _, log := range logs {
		if log.ConfigFile != configPath {
			t.Errorf("Expected config file %s, got %s", configPath, log.ConfigFile)
		}
		if log.Type == "access" {
			accessFound = true
		}
		if log.Type == "error" {
			errorFound = true
		}
	}

	if !accessFound || !errorFound {
		t.Error("Expected both access and error logs to be found")
	}

	// Second scan with only one log directive (error_log removed)
	content2 := []byte(`
server {
    listen 80;
    server_name example.com;
    
    access_log /var/log/nginx/access.log;
}
`)

	err = scanForLogDirectives(configPath, content2)
	if err != nil {
		t.Fatalf("Second scan failed: %v", err)
	}

	// Check that only access log remains
	logs = GetAllLogPaths()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log after second scan, got %d", len(logs))
	}

	if logs[0].Type != "access" {
		t.Errorf("Expected remaining log to be access log, got %s", logs[0].Type)
	}

	// Third scan with no log directives
	content3 := []byte(`
server {
    listen 80;
    server_name example.com;
}
`)

	err = scanForLogDirectives(configPath, content3)
	if err != nil {
		t.Fatalf("Third scan failed: %v", err)
	}

	// Check that no logs remain
	logs = GetAllLogPaths()
	if len(logs) != 0 {
		t.Fatalf("Expected 0 logs after third scan, got %d", len(logs))
	}
}

// TestScanForLogDirectivesMultipleConfigs tests that logs from different config files are handled independently
func TestScanForLogDirectivesMultipleConfigs(t *testing.T) {
	// Clear cache before test
	ClearLogCache()

	configPath1 := "/etc/nginx/sites-available/site1.conf"
	configPath2 := "/etc/nginx/sites-available/site2.conf"

	// Scan first config
	content1 := []byte(`
server {
    listen 80;
    server_name site1.com;
    access_log /var/log/nginx/site1_access.log;
}
`)

	err := scanForLogDirectives(configPath1, content1)
	if err != nil {
		t.Fatalf("First config scan failed: %v", err)
	}

	// Scan second config
	content2 := []byte(`
server {
    listen 80;
    server_name site2.com;
    access_log /var/log/nginx/site2_access.log;
}
`)

	err = scanForLogDirectives(configPath2, content2)
	if err != nil {
		t.Fatalf("Second config scan failed: %v", err)
	}

	// Should have 2 logs total
	logs := GetAllLogPaths()
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs from 2 configs, got %d", len(logs))
	}

	// Remove log from first config, should only affect that config
	emptyContent := []byte(`
server {
    listen 80;
    server_name site1.com;
}
`)

	err = scanForLogDirectives(configPath1, emptyContent)
	if err != nil {
		t.Fatalf("Empty config scan failed: %v", err)
	}

	// Should have 1 log remaining (from config2)
	logs = GetAllLogPaths()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log after removing from config1, got %d", len(logs))
	}

	if logs[0].ConfigFile != configPath2 {
		t.Errorf("Expected remaining log to be from config2 (%s), got %s", configPath2, logs[0].ConfigFile)
	}
}

// TestScanForLogDirectivesIgnoreComments tests that commented log directives are ignored
func TestScanForLogDirectivesIgnoreComments(t *testing.T) {
	// Clear cache before test
	ClearLogCache()

	configPath := "/etc/nginx/sites-available/test.conf"

	// Content with both active and commented log directives
	content := []byte(`
server {
    listen 80;
    server_name example.com;
    
    # This is a commented access log - should be ignored
    # access_log /var/log/nginx/commented_access.log;
    
    # Multi-line comment block
    #error_log /var/log/nginx/commented_error.log;
    
    # Active log directives (not commented)
    access_log /var/log/nginx/active_access.log;
    error_log /var/log/nginx/active_error.log;
    
    # Another commented directive with indentation
        # access_log /var/log/nginx/indented_comment.log;
    
    # Inline comment after directive should still work
    access_log /var/log/nginx/inline_comment.log; # this is active with comment
}
`)

	err := scanForLogDirectives(configPath, content)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should only find 3 active log directives (not the commented ones)
	logs := GetAllLogPaths()
	expectedCount := 3
	if len(logs) != expectedCount {
		t.Fatalf("Expected %d logs, got %d. Logs found: %+v", expectedCount, len(logs), logs)
	}

	// Verify the correct paths were found
	expectedPaths := map[string]bool{
		"/var/log/nginx/active_access.log":  false,
		"/var/log/nginx/active_error.log":   false,
		"/var/log/nginx/inline_comment.log": false,
	}

	for _, log := range logs {
		if _, exists := expectedPaths[log.Path]; !exists {
			t.Errorf("Unexpected log path found: %s", log.Path)
		} else {
			expectedPaths[log.Path] = true
		}
	}

	// Check that all expected paths were found
	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Expected log path not found: %s", path)
		}
	}

	// Verify no commented paths were included
	commentedPaths := []string{
		"/var/log/nginx/commented_access.log",
		"/var/log/nginx/commented_error.log",
		"/var/log/nginx/indented_comment.log",
	}

	for _, log := range logs {
		for _, commentedPath := range commentedPaths {
			if log.Path == commentedPath {
				t.Errorf("Commented log path should not be included: %s", commentedPath)
			}
		}
	}
}

// TestLogDirectiveRegex tests the regex pattern and comment filtering logic
func TestLogDirectiveRegex(t *testing.T) {
	testCases := []struct {
		name           string
		content        string
		expectedActive int // number of active (non-commented) matches expected
	}{
		{
			name:           "Active directives",
			content:        "access_log /var/log/nginx/access.log;\nerror_log /var/log/nginx/error.log;",
			expectedActive: 2,
		},
		{
			name:           "Commented directives",
			content:        "# access_log /var/log/nginx/access.log;\n#error_log /var/log/nginx/error.log;",
			expectedActive: 0,
		},
		{
			name:           "Mixed active and commented",
			content:        "access_log /var/log/nginx/access.log;\n# error_log /var/log/nginx/error.log;",
			expectedActive: 1,
		},
		{
			name:           "Indented comments",
			content:        "    # access_log /var/log/nginx/access.log;\n    error_log /var/log/nginx/error.log;",
			expectedActive: 1,
		},
		{
			name:           "Inline comments after directive",
			content:        "access_log /var/log/nginx/access.log; # this is a comment",
			expectedActive: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Find all matches using the regex
			matches := logDirectiveRegex.FindAllSubmatch([]byte(tc.content), -1)

			// Count how many are not commented
			activeCount := 0
			for _, match := range matches {
				if !isCommentedMatch([]byte(tc.content), match) {
					activeCount++
				}
			}

			if activeCount != tc.expectedActive {
				t.Errorf("Test '%s': expected %d active matches, got %d. Content: %s",
					tc.name, tc.expectedActive, activeCount, tc.content)
			}
		})
	}
}

// TestIsCommentedMatch tests the isCommentedMatch function directly
func TestIsCommentedMatch(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		matchStr    string
		isCommented bool
	}{
		{
			name:        "Not commented",
			content:     "access_log /var/log/nginx/access.log;",
			matchStr:    "access_log /var/log/nginx/access.log;",
			isCommented: false,
		},
		{
			name:        "Commented with #",
			content:     "# access_log /var/log/nginx/access.log;",
			matchStr:    "access_log /var/log/nginx/access.log;",
			isCommented: true,
		},
		{
			name:        "Commented with spaces and #",
			content:     "    # access_log /var/log/nginx/access.log;",
			matchStr:    "access_log /var/log/nginx/access.log;",
			isCommented: true,
		},
		{
			name:        "Not commented with spaces",
			content:     "    access_log /var/log/nginx/access.log;",
			matchStr:    "access_log /var/log/nginx/access.log;",
			isCommented: false,
		},
		{
			name:        "Inline comment after directive",
			content:     "access_log /var/log/nginx/access.log; # comment",
			matchStr:    "access_log /var/log/nginx/access.log;",
			isCommented: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake match to test with
			match := [][]byte{[]byte(tc.matchStr)}
			result := isCommentedMatch([]byte(tc.content), match)

			if result != tc.isCommented {
				t.Errorf("Test '%s': expected isCommented=%v, got %v. Content: %q, Match: %q",
					tc.name, tc.isCommented, result, tc.content, tc.matchStr)
			}
		})
	}
}
