package nginx

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// Site represents minimal site info needed for sandbox testing
type SandboxSite struct {
	Path string
}

// Stream represents minimal stream info needed for sandbox testing
type SandboxStream struct {
	Path string
}

// NamespaceInfo represents minimal namespace info for sandbox
type NamespaceInfo struct {
	ID         uint64
	Name       string
	DeployMode string
}

// SandboxTestConfigWithPaths tests nginx config in an isolated sandbox with provided paths
func SandboxTestConfigWithPaths(namespace *NamespaceInfo, sitePaths, streamPaths []string) (stdOut string, stdErr error) {
	mutex.Lock()
	defer mutex.Unlock()

	// If custom test command is set, use it (no sandbox support)
	if settings.NginxSettings.TestConfigCmd != "" {
		return execShell(settings.NginxSettings.TestConfigCmd)
	}

	// Skip local test for remote-only namespaces
	if namespace != nil && namespace.DeployMode == "remote" {
		return "Config validation skipped for remote-only namespace", nil
	}

	// Create sandbox and test
	sandbox, err := createSandbox(namespace, sitePaths, streamPaths)
	if err != nil {
		logger.Errorf("Failed to create sandbox: %v", err)
		return TestConfig() // Fallback to normal test
	}
	defer sandbox.Cleanup()

	// Test the sandbox config
	sbin := GetSbinPath()
	if sbin == "" {
		sbin = "nginx"
	}

	return execCommand(sbin, "-t", "-c", sandbox.ConfigPath)
}

// Sandbox represents an isolated nginx test environment
type Sandbox struct {
	Dir        string
	ConfigPath string
	Namespace  *NamespaceInfo
}

// createSandbox creates an isolated nginx configuration environment for testing
func createSandbox(namespace *NamespaceInfo, sitePaths, streamPaths []string) (*Sandbox, error) {
	// Create temp directory for sandbox
	tempDir, err := os.MkdirTemp("", "nginx-ui-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox temp dir: %w", err)
	}

	sandbox := &Sandbox{
		Dir:       tempDir,
		Namespace: namespace,
	}

	// Copy necessary directories to sandbox for complete isolation
	if err := copySandboxDependencies(tempDir); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to copy sandbox dependencies: %w", err)
	}

	// Generate sandbox nginx.conf
	configContent, err := generateSandboxConfig(namespace, sitePaths, streamPaths, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to generate sandbox config: %w", err)
	}

	// Write sandbox nginx.conf
	sandbox.ConfigPath = filepath.Join(tempDir, "nginx.conf")
	if err := os.WriteFile(sandbox.ConfigPath, []byte(configContent), 0644); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write sandbox config: %w", err)
	}

	logger.Debugf("Created sandbox at %s for namespace: %v", tempDir, namespace)
	return sandbox, nil
}

// copySandboxDependencies copies necessary config directories to sandbox
func copySandboxDependencies(sandboxDir string) error {
	confBase := GetConfPath()

	// Directories to copy for complete isolation
	dirsToCopy := []string{
		"conf.d",
		"modules-enabled",
		"snippets", // Common nginx snippets directory
	}

	for _, dir := range dirsToCopy {
		srcDir := filepath.Join(confBase, dir)
		dstDir := filepath.Join(sandboxDir, dir)

		// Check if source directory exists
		if !helper.FileExists(srcDir) {
			continue // Skip non-existent directories
		}

		// Create destination directory
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}

		// Copy all files from source to destination
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			logger.Warnf("Failed to read %s: %v, skipping", srcDir, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue // Skip subdirectories for now
			}

			srcFile := filepath.Join(srcDir, entry.Name())
			dstFile := filepath.Join(dstDir, entry.Name())

			content, err := os.ReadFile(srcFile)
			if err != nil {
				logger.Warnf("Failed to read %s: %v, skipping", srcFile, err)
				continue
			}

			if err := os.WriteFile(dstFile, content, 0644); err != nil {
				logger.Warnf("Failed to write %s: %v, skipping", dstFile, err)
				continue
			}
		}

		logger.Debugf("Copied %s to sandbox", dir)
	}

	// Also copy mime.types if exists
	mimeTypes := filepath.Join(confBase, "mime.types")
	if helper.FileExists(mimeTypes) {
		content, err := os.ReadFile(mimeTypes)
		if err == nil {
			os.WriteFile(filepath.Join(sandboxDir, "mime.types"), content, 0644)
		}
	}

	return nil
}

// Cleanup removes the sandbox directory
func (s *Sandbox) Cleanup() {
	if s.Dir != "" {
		if err := os.RemoveAll(s.Dir); err != nil {
			logger.Warnf("Failed to cleanup sandbox %s: %v", s.Dir, err)
		} else {
			logger.Debugf("Cleaned up sandbox: %s", s.Dir)
		}
	}
}

// generateSandboxConfig generates a minimal nginx.conf that only includes configs from specified paths
func generateSandboxConfig(namespace *NamespaceInfo, sitePaths, streamPaths []string, sandboxDir string) (string, error) {
	// Read the main nginx.conf to get basic structure
	mainConfPath := GetConfEntryPath()
	mainConf, err := os.ReadFile(mainConfPath)
	if err != nil {
		return "", fmt.Errorf("failed to read main nginx.conf: %w", err)
	}

	mainConfStr := string(mainConf)

	// Generate include patterns based on provided paths
	var includePatterns []string

	// Add site includes
	for _, sitePath := range sitePaths {
		siteEnabledPath := GetConfPath("sites-enabled", filepath.Base(sitePath))
		if helper.FileExists(siteEnabledPath) {
			includePatterns = append(includePatterns, fmt.Sprintf("    include %s;", siteEnabledPath))
		}
	}

	// Add stream includes
	for _, streamPath := range streamPaths {
		streamEnabledPath := GetConfPath("streams-enabled", filepath.Base(streamPath))
		if helper.FileExists(streamEnabledPath) {
			includePatterns = append(includePatterns, fmt.Sprintf("    include %s;", streamEnabledPath))
		}
	}

	// If no paths provided, test all enabled configs (original behavior)
	if len(includePatterns) == 0 {
		sitesEnabledDir := GetConfPath("sites-enabled")
		streamsEnabledDir := GetConfPath("streams-enabled")

		includePatterns = append(includePatterns, fmt.Sprintf("    include %s/*;", sitesEnabledDir))
		includePatterns = append(includePatterns, fmt.Sprintf("    include %s/*;", streamsEnabledDir))
	}

	// Replace include directives with sandbox-specific ones
	sandboxConf := replaceIncludeDirectives(mainConfStr, includePatterns, sandboxDir)

	return sandboxConf, nil
}

// replaceIncludeDirectives replaces only sites-enabled and streams-enabled includes
// Rewrites other includes (conf.d, mime.types, etc.) to use sandbox paths
func replaceIncludeDirectives(mainConf string, includePatterns []string, sandboxDir string) string {
	lines := strings.Split(mainConf, "\n")
	var result []string
	insideHTTP := false
	insideStream := false
	httpIncludesAdded := false
	streamIncludesAdded := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track http and stream blocks
		if strings.HasPrefix(trimmed, "http") && strings.Contains(trimmed, "{") {
			insideHTTP = true
			result = append(result, line)
			continue
		}
		if strings.HasPrefix(trimmed, "stream") && strings.Contains(trimmed, "{") {
			insideStream = true
			result = append(result, line)
			continue
		}

		// Handle include directives
		if strings.Contains(trimmed, "include") {
			isSitesEnabled := strings.Contains(trimmed, "sites-enabled")
			isStreamsEnabled := strings.Contains(trimmed, "streams-enabled")

			// If it's sites-enabled or streams-enabled, replace it
			if isSitesEnabled || isStreamsEnabled {
				// Add our sandbox-specific includes at the first occurrence
				if insideHTTP && isSitesEnabled && !httpIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					for _, pattern := range includePatterns {
						if strings.Contains(pattern, "sites-enabled") {
							result = append(result, pattern)
						}
					}
					httpIncludesAdded = true
				}
				if insideStream && isStreamsEnabled && !streamIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					for _, pattern := range includePatterns {
						if strings.Contains(pattern, "streams-enabled") {
							result = append(result, pattern)
						}
					}
					streamIncludesAdded = true
				}
				continue // Skip the original include line
			}

			// Rewrite other includes to use sandbox paths
			rewrittenLine := rewriteIncludePath(line, sandboxDir)
			result = append(result, rewrittenLine)
			continue
		}

		// Detect end of http/stream block
		if strings.Contains(line, "}") {
			if insideHTTP {
				// Add includes before closing http block if not added yet
				if !httpIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					for _, pattern := range includePatterns {
						if strings.Contains(pattern, "sites-enabled") {
							result = append(result, pattern)
						}
					}
					httpIncludesAdded = true
				}
				insideHTTP = false
			}
			if insideStream {
				// Add includes before closing stream block if not added yet
				if !streamIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					for _, pattern := range includePatterns {
						if strings.Contains(pattern, "streams-enabled") {
							result = append(result, pattern)
						}
					}
					streamIncludesAdded = true
				}
				insideStream = false
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// rewriteIncludePath rewrites include paths to use sandbox directory
func rewriteIncludePath(line, sandboxDir string) string {
	// Extract the include path using regex
	// Match: include /path/to/file; or include /path/*.conf;
	includeRegex := regexp.MustCompile(`include\s+([^;]+);`)
	matches := includeRegex.FindStringSubmatch(line)

	if len(matches) < 2 {
		return line // No match, return original
	}

	origPath := strings.TrimSpace(matches[1])
	confBase := GetConfPath()

	// Paths to rewrite to sandbox
	rewritePaths := map[string]string{
		filepath.Join(confBase, "conf.d"):         filepath.Join(sandboxDir, "conf.d"),
		filepath.Join(confBase, "modules-enabled"): filepath.Join(sandboxDir, "modules-enabled"),
		filepath.Join(confBase, "snippets"):        filepath.Join(sandboxDir, "snippets"),
		filepath.Join(confBase, "mime.types"):      filepath.Join(sandboxDir, "mime.types"),
	}

	// Check if path starts with any of the rewrite paths
	newPath := origPath
	for oldPrefix, newPrefix := range rewritePaths {
		if strings.HasPrefix(origPath, oldPrefix) {
			newPath = strings.Replace(origPath, oldPrefix, newPrefix, 1)
			break
		}
	}

	// Replace in the original line
	return strings.Replace(line, origPath, newPath, 1)
}
