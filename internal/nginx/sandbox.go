package nginx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

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

	// If namespace is nil, directly test in real directory (no sandbox)
	if namespace == nil {
		return TestConfig()
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

	// Copy full nginx conf directory to sandbox, excluding sites-* and streams-*
	if err := copyConfigBaseExceptSitesStreams(tempDir); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to copy base configs: %w", err)
	}

	// Ensure sandbox sub-directories exist for selected includes
	if err := os.MkdirAll(filepath.Join(tempDir, "sites-enabled"), 0755); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create sandbox sites-enabled: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "streams-enabled"), 0755); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create sandbox streams-enabled: %w", err)
	}

	// Collect and copy only enabled sites/streams for the given namespace
	siteFiles, streamFiles, err := collectAndCopyNamespaceEnabled(namespace, sitePaths, streamPaths, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to collect/copy namespace configs: %w", err)
	}

	// Generate sandbox nginx.conf
	configContent, err := generateSandboxConfig(namespace, siteFiles, streamFiles, tempDir)
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
func generateSandboxConfig(namespace *NamespaceInfo, siteFiles, streamFiles []string, sandboxDir string) (string, error) {
	// Read the main nginx.conf to get basic structure
	mainConfPath := GetConfEntryPath()
	mainConf, err := os.ReadFile(mainConfPath)
	if err != nil {
		return "", fmt.Errorf("failed to read main nginx.conf: %w", err)
	}

	mainConfStr := string(mainConf)

	// Generate include patterns based on provided paths
	siteIncludeLines := make([]string, 0, len(siteFiles))
	for _, f := range siteFiles {
		siteIncludeLines = append(siteIncludeLines, fmt.Sprintf("    include %s;", filepath.Join(sandboxDir, "sites-enabled", f)))
	}
	streamIncludeLines := make([]string, 0, len(streamFiles))
	for _, f := range streamFiles {
		streamIncludeLines = append(streamIncludeLines, fmt.Sprintf("    include %s;", filepath.Join(sandboxDir, "streams-enabled", f)))
	}

	// Replace include directives with sandbox-specific ones
	sandboxConf := replaceIncludeDirectives(mainConfStr, sandboxDir, siteIncludeLines, streamIncludeLines)

	return sandboxConf, nil
}

// replaceIncludeDirectives replaces only sites-enabled and streams-enabled includes
// Rewrites other includes to point to copied files under sandboxDir, preserving isolation.
func replaceIncludeDirectives(mainConf string, sandboxDir string, siteIncludeLines, streamIncludeLines []string) string {
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
					result = append(result, siteIncludeLines...)
					httpIncludesAdded = true
				}
				if insideStream && isStreamsEnabled && !streamIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					result = append(result, streamIncludeLines...)
					streamIncludesAdded = true
				}
				continue // Skip the original include line
			}

			// Rewrite includes to sandbox paths
			normalized := rewriteIncludeLineToSandbox(line, sandboxDir)
			if normalized != "" {
				result = append(result, normalized)
			}
			continue
		}

		// Detect end of http/stream block
		if strings.Contains(line, "}") {
			if insideHTTP {
				// Add includes before closing http block if not added yet
				if !httpIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					result = append(result, siteIncludeLines...)
					httpIncludesAdded = true
				}
				insideHTTP = false
			}
			if insideStream {
				// Add includes before closing stream block if not added yet
				if !streamIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					result = append(result, streamIncludeLines...)
					streamIncludesAdded = true
				}
				insideStream = false
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// rewriteIncludeLineToSandbox rewrites include lines to point to files/directories inside sandboxDir.
// If an include path is relative, it will be rewritten relative to the nginx conf dir inside sandbox.
func rewriteIncludeLineToSandbox(line string, sandboxDir string) string {
	includeRegex := regexp.MustCompile(`(?i)include\s+([^;#]+);`)
	matches := includeRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return line
	}
	path := strings.TrimSpace(matches[1])

	confBase := GetConfPath()
	var rewritten string
	if filepath.IsAbs(path) {
		// If absolute under confBase, map to sandbox
		if helper.IsUnderDirectory(path, confBase) {
			rel, err := filepath.Rel(confBase, path)
			if err == nil {
				rewritten = filepath.Join(sandboxDir, rel)
			}
		}
	} else {
		// Relative includes should point inside sandbox conf root
		rewritten = filepath.Join(sandboxDir, path)
	}
	if rewritten == "" {
		rewritten = path
	}
	trimmed := includeRegex.ReplaceAllString(line, "include "+rewritten+";")
	return trimmed
}

// collectAndCopyNamespaceEnabled collects and copies enabled site/stream configs based on provided paths.
// It rewrites relative includes to absolute, and writes them into sandboxDir/{sites-enabled,streams-enabled}.
// Returns the written file names.
func collectAndCopyNamespaceEnabled(_ *NamespaceInfo, sitePaths, streamPaths []string, sandboxDir string) (siteFiles, streamFiles []string, err error) {
	// Helper to process and write a single config by kind and name
	readSourceAndWrite := func(kind, name string) (writtenName string, wErr error) {
		var enabledCandidates []string
		switch kind {
		case "site":
			enabledCandidates = []string{
				GetConfSymlinkPath(GetConfPath("sites-enabled", name)),
				GetConfPath("sites-enabled", name),
			}
		case "stream":
			enabledCandidates = []string{
				GetConfSymlinkPath(GetConfPath("streams-enabled", name)),
				GetConfPath("streams-enabled", name),
			}
		}
		var enabledPath string
		for _, cand := range enabledCandidates {
			if helper.FileExists(cand) {
				enabledPath = cand
				break
			}
		}
		if enabledPath == "" {
			return "", nil // not enabled, skip silently
		}

		// Determine source file: prefer the symlink target if possible; fallback to *-available
		srcPath := enabledPath
		if fi, lErr := os.Lstat(enabledPath); lErr == nil && (fi.Mode()&os.ModeSymlink) != 0 {
			if target, rErr := os.Readlink(enabledPath); rErr == nil {
				// If target is relative, resolve against enabled dir
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(enabledPath), target)
				}
				srcPath = target
			}
		}
		if kind == "site" && !helper.FileExists(srcPath) {
			srcPath = GetConfPath("sites-available", name)
		}
		if kind == "stream" && !helper.FileExists(srcPath) {
			srcPath = GetConfPath("streams-available", name)
		}
		content, rErr := os.ReadFile(srcPath)
		if rErr != nil {
			return "", fmt.Errorf("read %s content %s: %w", kind, srcPath, rErr)
		}

		// Rewrite include lines to sandbox paths (resolve relative to source dir first)
		absRewriter := regexp.MustCompile(`(?m)^[ \t]*include\s+([^;#]+);`)
		rewritten := absRewriter.ReplaceAllStringFunc(string(content), func(m string) string {
			return normalizeIncludeLineRelativeTo(m, filepath.Dir(srcPath), sandboxDir)
		})

		// Compute destination file name respecting platform symlink naming
		var destName string
		switch kind {
		case "site":
			destName = filepath.Base(GetConfSymlinkPath(GetConfPath("sites-enabled", name)))
		case "stream":
			destName = filepath.Base(GetConfSymlinkPath(GetConfPath("streams-enabled", name)))
		}

		destDir := filepath.Join(sandboxDir, kind+"s-enabled")
		if err := os.WriteFile(filepath.Join(destDir, destName), []byte(rewritten), 0644); err != nil {
			return "", fmt.Errorf("write sandbox %s: %w", kind, err)
		}
		return destName, nil
	}

	// Process sites based on provided sitePaths
	for _, sp := range sitePaths {
		name := filepath.Base(sp)
		if written, wErr := readSourceAndWrite("site", name); wErr != nil {
			return nil, nil, wErr
		} else if written != "" {
			siteFiles = append(siteFiles, written)
		}
	}

	// Process streams based on provided streamPaths
	for _, st := range streamPaths {
		name := filepath.Base(st)
		if written, wErr := readSourceAndWrite("stream", name); wErr != nil {
			return nil, nil, wErr
		} else if written != "" {
			streamFiles = append(streamFiles, written)
		}
	}

	return siteFiles, streamFiles, nil
}

// normalizeIncludeLineRelativeTo rewrites a single include line:
// - resolves relative paths against baseDir
// - if the resolved path is under confBase, map to sandboxDir mirror; else keep as is
func normalizeIncludeLineRelativeTo(line, baseDir, sandboxDir string) string {
	includeRegex := regexp.MustCompile(`(?i)include\s+([^;#]+);`)
	matches := includeRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return line
	}
	path := strings.TrimSpace(matches[1])

	// If relative, make absolute to source file dir
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Clean(filepath.Join(baseDir, resolved))
	}
	confBase := GetConfPath()
	if helper.IsUnderDirectory(resolved, confBase) {
		if rel, err := filepath.Rel(confBase, resolved); err == nil {
			resolved = filepath.Join(sandboxDir, rel)
		}
	}
	return includeRegex.ReplaceAllString(line, "include "+resolved+";")
}

// copyConfigBaseExceptSitesStreams copies the entire nginx conf directory into sandboxDir,
// excluding any paths under sites-* and streams-* and skipping the entry nginx.conf (we generate our own).
func copyConfigBaseExceptSitesStreams(sandboxDir string) error {
	confBase := GetConfPath()
	entry := GetConfEntryPath()

	copyFile := func(src, dst string, mode fs.FileMode) error {
		parent := filepath.Dir(dst)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return err
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0644)
	}

	return filepath.WalkDir(confBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rErr := filepath.Rel(confBase, path)
		if rErr != nil {
			return rErr
		}
		if rel == "." {
			return nil
		}
		// Skip blacklisted directories
		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, "sites-") || strings.HasPrefix(base, "streams-") {
				return filepath.SkipDir
			}
			// Create directory in sandbox
			return os.MkdirAll(filepath.Join(sandboxDir, rel), 0755)
		}
		// Skip entry nginx.conf to avoid overwriting generated file
		if path == entry && filepath.Base(path) == "nginx.conf" {
			return nil
		}
		// Copy regular file (follow symlinks by reading content)
		dst := filepath.Join(sandboxDir, rel)
		info, sErr := os.Lstat(path)
		if sErr != nil {
			return sErr
		}
		return copyFile(path, dst, info.Mode())
	})
}
