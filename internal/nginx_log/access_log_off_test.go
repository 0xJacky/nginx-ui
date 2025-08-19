package nginx_log

import (
	"testing"
)

// TestAccessLogOffDirective tests that "access_log off;" directives are properly ignored
func TestAccessLogOffDirective(t *testing.T) {
	// Clear cache before test
	ClearLogCache()

	configPath := "/etc/nginx/sites-available/test.conf"

	// Test 1: Normal logs without "off"
	t.Run("Normal logs", func(t *testing.T) {
		ClearLogCache()
		content := []byte(`
server {
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;
}`)
		
		err := scanForLogDirectives(configPath, content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		
		logs := GetAllLogPaths()
		if len(logs) != 2 {
			t.Errorf("Expected 2 logs, got %d", len(logs))
		}
	})

	// Test 2: Logs with "off" directive
	t.Run("With off directive", func(t *testing.T) {
		ClearLogCache()
		content := []byte(`
server {
    access_log /var/log/nginx/access.log;
    access_log off;
    error_log off;
    error_log /var/log/nginx/error.log;
}`)
		
		err := scanForLogDirectives(configPath, content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		
		logs := GetAllLogPaths()
		if len(logs) != 2 {
			t.Errorf("Expected 2 logs (ignoring 'off'), got %d", len(logs))
			for _, log := range logs {
				t.Logf("Found: %s (%s)", log.Path, log.Type)
			}
		}
		
		// Verify "off" is not treated as a path
		for _, log := range logs {
			if log.Path == "off" {
				t.Errorf("'off' should not be treated as a log path")
			}
		}
	})

	// Test 3: Only "off" directives
	t.Run("Only off directives", func(t *testing.T) {
		ClearLogCache()
		content := []byte(`
server {
    access_log off;
    error_log off;
}`)
		
		err := scanForLogDirectives(configPath, content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		
		logs := GetAllLogPaths()
		if len(logs) != 0 {
			t.Errorf("Expected 0 logs (all 'off'), got %d", len(logs))
			for _, log := range logs {
				t.Logf("Found: %s (%s)", log.Path, log.Type)
			}
		}
	})

	// Test 4: Mixed with format parameters
	t.Run("With format parameters", func(t *testing.T) {
		ClearLogCache()
		content := []byte(`
server {
    access_log /var/log/nginx/access.log combined buffer=32k;
    access_log off;
    error_log /var/log/nginx/error.log warn;
}`)
		
		err := scanForLogDirectives(configPath, content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		
		logs := GetAllLogPaths()
		if len(logs) != 2 {
			t.Errorf("Expected 2 logs, got %d", len(logs))
			for _, log := range logs {
				t.Logf("Found: %s (%s)", log.Path, log.Type)
			}
		}
	})
}