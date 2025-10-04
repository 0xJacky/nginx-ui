package nginx

import (
	"os"
	"strings"
	"testing"
)

func TestCreateSandbox(t *testing.T) {
	namespaceInfo := &NamespaceInfo{
		ID:   1,
		Name: "test-namespace",
	}

	sitePaths := []string{"site1.conf", "site2.conf"}
	streamPaths := []string{"stream1.conf"}

	sandbox, err := createSandbox(namespaceInfo, sitePaths, streamPaths)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	// Verify sandbox directory exists
	if _, err := os.Stat(sandbox.Dir); os.IsNotExist(err) {
		t.Errorf("Sandbox directory does not exist: %s", sandbox.Dir)
	}

	// Verify config file exists
	if _, err := os.Stat(sandbox.ConfigPath); os.IsNotExist(err) {
		t.Errorf("Sandbox config file does not exist: %s", sandbox.ConfigPath)
	}

	// Verify namespace info
	if sandbox.Namespace.ID != 1 {
		t.Errorf("Expected namespace ID 1, got %d", sandbox.Namespace.ID)
	}
}

func TestSandboxCleanup(t *testing.T) {
	sandbox, err := createSandbox(nil, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	sandboxDir := sandbox.Dir

	// Cleanup
	sandbox.Cleanup()

	// Verify directory is removed
	if _, err := os.Stat(sandboxDir); !os.IsNotExist(err) {
		t.Errorf("Sandbox directory still exists after cleanup: %s", sandboxDir)
	}
}

func TestGenerateSandboxConfig(t *testing.T) {
	// Skip this test as it requires mocking GetConfEntryPath
	// The logic is tested in TestReplaceIncludeDirectives instead
	t.Skip("Skipping - requires dependency injection refactoring")
}

func TestReplaceIncludeDirectives(t *testing.T) {
	tests := []struct {
		name            string
		mainConf        string
		includePatterns []string
		expectContains  []string
		expectNotContain []string
	}{
		{
			name: "Replace HTTP includes",
			mainConf: `http {
    include /etc/nginx/sites-enabled/*;
}`,
			includePatterns: []string{
				"    include /etc/nginx/sites-enabled/site1.conf;",
				"    include /etc/nginx/sites-enabled/site2.conf;",
			},
			expectContains: []string{
				"include /etc/nginx/sites-enabled/site1.conf",
				"include /etc/nginx/sites-enabled/site2.conf",
				"Sandbox-specific includes",
			},
			expectNotContain: []string{
				"include /etc/nginx/sites-enabled/*",
			},
		},
		{
			name: "Replace Stream includes",
			mainConf: `stream {
    include /etc/nginx/streams-enabled/*;
}`,
			includePatterns: []string{
				"    include /etc/nginx/streams-enabled/stream1.conf;",
			},
			expectContains: []string{
				"include /etc/nginx/streams-enabled/stream1.conf",
				"Sandbox-specific includes",
			},
			expectNotContain: []string{
				"include /etc/nginx/streams-enabled/*",
			},
		},
		{
			name: "Rewrite other includes to sandbox",
			mainConf: `http {
    include /etc/nginx/mime.types;
    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}`,
			includePatterns: []string{
				"    include /etc/nginx/sites-enabled/site1.conf;",
			},
			expectContains: []string{
				"include /tmp/test-sandbox/mime.types", // Rewritten to sandbox
				"include /tmp/test-sandbox/conf.d/*.conf", // Rewritten to sandbox
				"include /etc/nginx/sites-enabled/site1.conf",
			},
			expectNotContain: []string{
				"include /etc/nginx/sites-enabled/*",
				"include /etc/nginx/mime.types", // Should be rewritten
				"include /etc/nginx/conf.d/*.conf", // Should be rewritten
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandboxDir := "/tmp/test-sandbox"
			result := replaceIncludeDirectives(tt.mainConf, tt.includePatterns, sandboxDir)

			for _, expected := range tt.expectContains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, but it doesn't.\nResult:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.expectNotContain {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected result NOT to contain %q, but it does.\nResult:\n%s", notExpected, result)
				}
			}
		})
	}
}

func TestReplaceIncludeDirectivesEdgeCases(t *testing.T) {
	t.Run("Empty include patterns", func(t *testing.T) {
		mainConf := `http {
    include /etc/nginx/sites-enabled/*;
}`
		result := replaceIncludeDirectives(mainConf, []string{}, "/tmp/test-sandbox")

		// Should still add comment but no includes
		if !strings.Contains(result, "Sandbox-specific includes") {
			t.Error("Expected sandbox comment even with empty patterns")
		}
	})

	t.Run("No http or stream blocks", func(t *testing.T) {
		mainConf := `events {
    worker_connections 1024;
}`
		includePatterns := []string{"    include /etc/nginx/sites-enabled/site1.conf;"}
		result := replaceIncludeDirectives(mainConf, includePatterns, "/tmp/test-sandbox")

		// Should preserve original config
		if !strings.Contains(result, "worker_connections 1024") {
			t.Error("Original config not preserved when no http/stream blocks")
		}
	})

	t.Run("Nested braces", func(t *testing.T) {
		mainConf := `http {
    server {
        location / {
            return 200;
        }
    }
    include /etc/nginx/sites-enabled/*;
}`
		includePatterns := []string{"    include /etc/nginx/sites-enabled/site1.conf;"}
		result := replaceIncludeDirectives(mainConf, includePatterns, "/tmp/test-sandbox")

		// Should preserve nested structure
		if !strings.Contains(result, "location /") {
			t.Error("Nested location directive not preserved")
		}

		// Should replace include
		if strings.Contains(result, "include /etc/nginx/sites-enabled/*") {
			t.Error("Generic include should be replaced even with nested braces")
		}
	})
}

func TestSandboxTestConfigWithPaths(t *testing.T) {
	// Skip this integration test - requires nginx installation and proper setup
	t.Skip("Skipping integration test - requires nginx binary and proper configuration")
}

func BenchmarkCreateSandbox(b *testing.B) {
	namespaceInfo := &NamespaceInfo{
		ID:   1,
		Name: "bench-namespace",
	}

	sitePaths := []string{"site1.conf", "site2.conf", "site3.conf"}
	streamPaths := []string{"stream1.conf"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sandbox, err := createSandbox(namespaceInfo, sitePaths, streamPaths)
		if err != nil {
			b.Fatalf("Failed to create sandbox: %v", err)
		}
		sandbox.Cleanup()
	}
}

func BenchmarkReplaceIncludeDirectives(b *testing.B) {
	mainConf := `
events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;

    server {
        listen 80;
        server_name default;
    }
}

stream {
    include /etc/nginx/streams-enabled/*;
}
`
	includePatterns := []string{
		"    include /etc/nginx/sites-enabled/site1.conf;",
		"    include /etc/nginx/sites-enabled/site2.conf;",
		"    include /etc/nginx/sites-enabled/site3.conf;",
		"    include /etc/nginx/streams-enabled/stream1.conf;",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = replaceIncludeDirectives(mainConf, includePatterns, "/tmp/test-sandbox")
	}
}
