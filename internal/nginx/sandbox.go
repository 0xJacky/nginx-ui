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

// SandboxTestConfigWithPaths tests nginx config in an isolated sandbox with provided paths.
func SandboxTestConfigWithPaths(namespace *NamespaceInfo, sitePaths, streamPaths []string) TestConfigResult {
	// If custom test command is set, use it (no sandbox support)
	if settings.NginxSettings.TestConfigCmd != "" {
		mutex.Lock()
		defer mutex.Unlock()
		stdOut, stdErr := execShell(settings.NginxSettings.TestConfigCmd)
		result := NewTestConfigResult(stdOut, stdErr, TestScopeNamespaceSandbox, SandboxStatusSkipped)
		result.Message = strings.TrimSpace(strings.Join([]string{
			"Sandbox validation skipped because a custom test command is configured.",
			result.Message,
		}, "\n"))
		return result
	}

	// Skip local test for remote-only namespaces
	if namespace != nil && namespace.DeployMode == "remote" {
		return TestConfigResult{
			Message:       "Config validation skipped for remote-only namespace",
			Level:         Notice,
			TestScope:     TestScopeNamespaceSandbox,
			SandboxStatus: SandboxStatusSkipped,
		}
	}

	// If namespace is nil, directly test in real directory (no sandbox)
	if namespace == nil {
		stdOut, stdErr := TestConfig()
		return NewTestConfigResult(stdOut, stdErr, TestScopeGlobal, "")
	}

	// Create sandbox and test
	sandbox, err := createSandbox(namespace, sitePaths, streamPaths)
	if err != nil {
		logger.Errorf("Failed to create sandbox: %v", err)
		return NewSandboxBuildFailureResult(err)
	}
	defer sandbox.Cleanup()

	// Test the sandbox config under the same global lock used by other control commands
	mutex.Lock()
	defer mutex.Unlock()

	sbin := GetSbinPath()
	if sbin == "" {
		sbin = "nginx"
	}

	stdOut, stdErr := execCommand(sbin, "-t", "-c", sandbox.ConfigPath)
	return NewTestConfigResult(stdOut, stdErr, TestScopeNamespaceSandbox, SandboxStatusOK)
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
	builder := newSandboxBuilder(tempDir)

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
	siteFiles, streamFiles, err := collectAndCopyNamespaceEnabled(namespace, sitePaths, streamPaths, builder)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to collect/copy namespace configs: %w", err)
	}

	// Generate sandbox nginx.conf
	configContent, err := generateSandboxConfig(namespace, siteFiles, streamFiles, builder)
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

// generateSandboxConfig generates a minimal nginx.conf that only includes configs from specified paths.
func generateSandboxConfig(namespace *NamespaceInfo, siteFiles, streamFiles []string, builder *sandboxBuilder) (string, error) {
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
		siteIncludeLines = append(siteIncludeLines, fmt.Sprintf("    include %s;", filepath.Join(builder.sandboxDir, "sites-enabled", f)))
	}
	streamIncludeLines := make([]string, 0, len(streamFiles))
	for _, f := range streamFiles {
		streamIncludeLines = append(streamIncludeLines, fmt.Sprintf("    include %s;", filepath.Join(builder.sandboxDir, "streams-enabled", f)))
	}

	// Replace include directives with sandbox-specific ones
	sandboxConf, err := replaceIncludeDirectives(mainConfStr, mainConfPath, builder, siteIncludeLines, streamIncludeLines)
	if err != nil {
		return "", err
	}

	return sandboxConf, nil
}

// replaceIncludeDirectives replaces only sites-enabled and streams-enabled includes.
// Rewrites other includes to point to copied files under sandboxDir, preserving isolation.
func replaceIncludeDirectives(mainConf string, sourcePath string, builder *sandboxBuilder, siteIncludeLines, streamIncludeLines []string) (string, error) {
	lines := strings.Split(mainConf, "\n")
	var result []string
	httpDepth := 0
	streamDepth := 0
	httpIncludesAdded := false
	streamIncludesAdded := false

	includeRx := regexp.MustCompile(`(?i)^\s*include\s+([^;#]+);`)
	httpOpenRx := regexp.MustCompile(`(?i)^\s*http\s*\{`)
	streamOpenRx := regexp.MustCompile(`(?i)^\s*stream\s*\{`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip processing for comment-only lines
		if strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}

		// Detect opening of http/stream blocks
		if httpOpenRx.MatchString(line) {
			httpDepth = 1
			httpIncludesAdded = false
			result = append(result, line)
			continue
		}
		if streamOpenRx.MatchString(line) {
			streamDepth = 1
			streamIncludesAdded = false
			result = append(result, line)
			continue
		}

		// Handle include directives (non-comment)
		if includeRx.MatchString(line) {
			isSitesEnabled := strings.Contains(line, "sites-enabled")
			isStreamsEnabled := strings.Contains(line, "streams-enabled")

			// If it's sites-enabled or streams-enabled, replace it
			if isSitesEnabled || isStreamsEnabled {
				// Add our sandbox-specific includes at the first occurrence
				if httpDepth > 0 && isSitesEnabled && !httpIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					result = append(result, siteIncludeLines...)
					httpIncludesAdded = true
				}
				if streamDepth > 0 && isStreamsEnabled && !streamIncludesAdded {
					result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
					result = append(result, streamIncludeLines...)
					streamIncludesAdded = true
				}
				// Skip the original include line
				continue
			}

			// Rewrite other includes to sandbox paths
			normalized, err := builder.rewriteIncludeLine(line, filepath.Dir(sourcePath))
			if err != nil {
				return "", err
			}
			if normalized != "" {
				result = append(result, normalized)
			}
			continue
		}

		// Before appending this line, check if it closes http/stream top-level block.
		openCount := strings.Count(line, "{")
		closeCount := strings.Count(line, "}")

		// If current httpDepth will reach zero after this line, inject includes BEFORE the closing brace line.
		if httpDepth > 0 {
			newDepth := httpDepth + openCount - closeCount
			if newDepth == 0 && !httpIncludesAdded {
				result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
				result = append(result, siteIncludeLines...)
				httpIncludesAdded = true
			}
			httpDepth = newDepth
		}
		if streamDepth > 0 {
			newDepth := streamDepth + openCount - closeCount
			if newDepth == 0 && !streamIncludesAdded {
				result = append(result, "    # Sandbox-specific includes (generated for isolated testing)")
				result = append(result, streamIncludeLines...)
				streamIncludesAdded = true
			}
			streamDepth = newDepth
		}

		// Append current line
		result = append(result, line)
	}

	return strings.Join(result, "\n"), nil
}

// collectAndCopyNamespaceEnabled collects and copies enabled site/stream configs based on provided paths.
// It rewrites relative includes to absolute, and writes them into sandboxDir/{sites-enabled,streams-enabled}.
// Returns the written file names.
func collectAndCopyNamespaceEnabled(_ *NamespaceInfo, sitePaths, streamPaths []string, builder *sandboxBuilder) (siteFiles, streamFiles []string, err error) {
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
		rewritten, rErr := builder.rewriteConfigContent(string(content), srcPath)
		if rErr != nil {
			return "", fmt.Errorf("rewrite sandbox %s: %w", kind, rErr)
		}

		// Compute destination file name respecting platform symlink naming
		var destName string
		switch kind {
		case "site":
			destName = filepath.Base(GetConfSymlinkPath(GetConfPath("sites-enabled", name)))
		case "stream":
			destName = filepath.Base(GetConfSymlinkPath(GetConfPath("streams-enabled", name)))
		}

		destDir := filepath.Join(builder.sandboxDir, kind+"s-enabled")
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

type sandboxBuilder struct {
	sandboxDir string
	confBase   string
	mirrored   map[string]bool
}

func newSandboxBuilder(sandboxDir string) *sandboxBuilder {
	return &sandboxBuilder{
		sandboxDir: sandboxDir,
		confBase:   GetConfPath(),
		mirrored:   map[string]bool{},
	}
}

func (b *sandboxBuilder) rewriteConfigContent(content string, sourcePath string) (string, error) {
	includeRegex := regexp.MustCompile(`(?m)^[ \t]*include\s+([^;#]+);`)
	var rewriteErr error

	rewritten := includeRegex.ReplaceAllStringFunc(content, func(match string) string {
		if rewriteErr != nil {
			return match
		}

		line, err := b.rewriteIncludeLine(match, filepath.Dir(sourcePath))
		if err != nil {
			rewriteErr = err
			return match
		}

		return line
	})

	if rewriteErr != nil {
		return "", rewriteErr
	}

	return rewritten, nil
}

func (b *sandboxBuilder) rewriteIncludeLine(line string, baseDir string) (string, error) {
	includeRegex := regexp.MustCompile(`(?i)include\s+([^;#]+);`)
	matches := includeRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return line, nil
	}

	includePath := strings.TrimSpace(matches[1])
	resolvedPath, matchedFiles, err := b.resolveIncludePath(includePath, baseDir)
	if err != nil {
		return "", err
	}

	if helper.IsUnderDirectory(resolvedPath, b.confBase) {
		for _, matchedFile := range matchedFiles {
			if err := b.mirrorDependency(matchedFile); err != nil {
				return "", err
			}
		}

		rel, err := filepath.Rel(b.confBase, resolvedPath)
		if err == nil {
			resolvedPath = filepath.Join(b.sandboxDir, rel)
		}
	}

	return includeRegex.ReplaceAllString(line, "include "+resolvedPath+";"), nil
}

func (b *sandboxBuilder) resolveIncludePath(includePath string, baseDir string) (string, []string, error) {
	if filepath.IsAbs(includePath) {
		matches, err := matchIncludePattern(includePath)
		if err != nil {
			return "", nil, err
		}
		if len(matches) == 0 && helper.IsUnderDirectory(includePath, b.confBase) {
			return "", nil, newSandboxIncludeError(baseDir, includePath)
		}
		return includePath, matches, nil
	}

	candidates := uniqueSandboxCandidates(
		filepath.Clean(filepath.Join(baseDir, includePath)),
		filepath.Clean(filepath.Join(b.confBase, includePath)),
	)

	for _, candidate := range candidates {
		matches, err := matchIncludePattern(candidate)
		if err != nil {
			return "", nil, err
		}
		if len(matches) > 0 {
			return candidate, matches, nil
		}
	}

	if len(candidates) == 0 {
		return includePath, nil, nil
	}

	return "", nil, newSandboxIncludeError(baseDir, includePath)
}

func (b *sandboxBuilder) mirrorDependency(sourcePath string) error {
	sourcePath = filepath.Clean(sourcePath)
	if !helper.IsUnderDirectory(sourcePath, b.confBase) {
		return nil
	}
	if b.mirrored[sourcePath] {
		return nil
	}
	b.mirrored[sourcePath] = true

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("failed to read sandbox dependency %s: %v", sourcePath, err),
		}
	}

	rewritten, err := b.rewriteConfigContent(string(data), sourcePath)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(b.confBase, sourcePath)
	if err != nil {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("failed to resolve sandbox dependency path %s: %v", sourcePath, err),
		}
	}

	destPath := filepath.Join(b.sandboxDir, rel)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("failed to create sandbox dependency dir for %s: %v", sourcePath, err),
		}
	}

	if err := os.WriteFile(destPath, []byte(rewritten), 0644); err != nil {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("failed to write sandbox dependency %s: %v", sourcePath, err),
		}
	}

	return nil
}

func matchIncludePattern(pattern string) ([]string, error) {
	if strings.ContainsAny(pattern, "*?[") {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, &SandboxBuildError{
				Category: ErrorCategorySandboxBuildError,
				Message:  fmt.Sprintf("invalid include pattern %s: %v", pattern, err),
			}
		}

		var existing []string
		for _, match := range matches {
			info, statErr := os.Stat(match)
			if statErr == nil && !info.IsDir() {
				existing = append(existing, match)
			}
		}

		return existing, nil
	}

	info, err := os.Stat(pattern)
	if err != nil || info.IsDir() {
		return nil, nil
	}

	return []string{pattern}, nil
}

func uniqueSandboxCandidates(paths ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(paths))

	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}

	return result
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
		base := filepath.Base(path)

		// Handle symlinked entries (including symlinks to directories such as /etc/nginx/modules)
		lstatInfo, lErr := os.Lstat(path)
		if lErr != nil {
			return lErr
		}
		if lstatInfo.Mode()&os.ModeSymlink != 0 {
			// Respect blacklist for sites/streams even when symlinked
			if strings.HasPrefix(base, "sites-") || strings.HasPrefix(base, "streams-") {
				return nil
			}
			target, tErr := os.Readlink(path)
			if tErr != nil {
				return tErr
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			// If the symlink points to a directory, recreate the symlink and stop processing this entry
			if tStat, sErr := os.Stat(target); sErr == nil && tStat.IsDir() {
				dst := filepath.Join(sandboxDir, rel)
				if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
					return mkErr
				}
				// Ignore existing symlink if already created
				if syErr := os.Symlink(target, dst); syErr != nil && !os.IsExist(syErr) {
					return syErr
				}
				return nil
			}
		}

		// Skip blacklisted directories
		if d.IsDir() {
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
		return copyFile(path, dst, lstatInfo.Mode())
	})
}
