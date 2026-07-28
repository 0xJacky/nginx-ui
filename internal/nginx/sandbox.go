package nginx

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
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
	// Remote namespaces have no local configuration to validate.
	if namespace != nil && namespace.DeployMode == "remote" {
		return TestConfigResult{
			Level:         Notice,
			TestScope:     TestScopeNamespaceSandbox,
			SandboxStatus: SandboxStatusSkipped,
			SandboxReason: SandboxReasonRemoteNamespace,
		}
	}

	// A process-local temporary directory is not visible inside a separate Nginx container.
	if namespace != nil && settings.NginxSettings.RunningInAnotherContainer() {
		return TestConfigResult{
			Level:         Notice,
			TestScope:     TestScopeNamespaceSandbox,
			SandboxStatus: SandboxStatusSkipped,
			SandboxReason: SandboxReasonSeparateContainer,
		}
	}

	// If a custom test command is set, use it without claiming sandbox coverage.
	if settings.NginxSettings.TestConfigCmd != "" {
		commandMutex.Lock()
		defer commandMutex.Unlock()
		stdOut, stdErr := execShell(settings.NginxSettings.TestConfigCmd)
		result := NewTestConfigResult(stdOut, stdErr, TestScopeNamespaceSandbox, SandboxStatusSkipped)
		result.SandboxReason = SandboxReasonCustomTestCommand
		return result
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
	commandMutex.Lock()
	defer commandMutex.Unlock()

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
		siteIncludeLines = append(siteIncludeLines, "    include "+quoteNginxPath(filepath.Join(builder.sandboxDir, "sites-enabled", f))+";")
	}
	streamIncludeLines := make([]string, 0, len(streamFiles))
	for _, f := range streamFiles {
		streamIncludeLines = append(streamIncludeLines, "    include "+quoteNginxPath(filepath.Join(builder.sandboxDir, "streams-enabled", f))+";")
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
	rewritten, err := builder.rewriteConfigContent(mainConf, sourcePath)
	if err != nil {
		return "", err
	}

	tokens, err := tokenizeNginxConfig(rewritten)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	stack := make([]string, 0, 8)
	lastWrite := 0
	statementStart := 0
	httpIncludesAdded := false
	streamIncludesAdded := false

	for i, token := range tokens {
		switch token.value {
		case "{":
			blockName := ""
			if statementStart < i {
				blockName = strings.ToLower(tokens[statementStart].value)
			}
			stack = append(stack, blockName)
			statementStart = i + 1
		case "}":
			if len(stack) > 0 {
				blockName := stack[len(stack)-1]
				if len(stack) == 1 && blockName == "http" && !httpIncludesAdded && len(siteIncludeLines) > 0 {
					result.WriteString(rewritten[lastWrite:token.start])
					writeSandboxIncludes(&result, siteIncludeLines)
					lastWrite = token.start
					httpIncludesAdded = true
				}
				if len(stack) == 1 && blockName == "stream" && !streamIncludesAdded && len(streamIncludeLines) > 0 {
					result.WriteString(rewritten[lastWrite:token.start])
					writeSandboxIncludes(&result, streamIncludeLines)
					lastWrite = token.start
					streamIncludesAdded = true
				}
				stack = stack[:len(stack)-1]
			}
			statementStart = i + 1
		case ";":
			statementStart = i + 1
		}
	}
	result.WriteString(rewritten[lastWrite:])

	if len(siteIncludeLines) > 0 && !httpIncludesAdded {
		return "", newSandboxStructureError("selected site configurations were not emitted inside an http block")
	}
	if len(streamIncludeLines) > 0 && !streamIncludesAdded {
		return "", newSandboxStructureError("selected stream configurations were not emitted inside a stream block")
	}

	return result.String(), nil
}

func writeSandboxIncludes(result *strings.Builder, includeLines []string) {
	result.WriteString("    # Sandbox-specific includes (generated for isolated testing)\n")
	result.WriteString(strings.Join(includeLines, "\n"))
	result.WriteByte('\n')
}

// collectAndCopyNamespaceEnabled collects and copies enabled site/stream configs based on provided paths.
// It rewrites relative includes to absolute, and writes them into sandboxDir/{sites-enabled,streams-enabled}.
// Returns the written file names.
func collectAndCopyNamespaceEnabled(_ *NamespaceInfo, sitePaths, streamPaths []string, builder *sandboxBuilder) (siteFiles, streamFiles []string, err error) {
	const maintenanceSuffix = "_nginx_ui_maintenance"

	readSourceAndWrite := func(kind, relativePath string) (writtenName string, wErr error) {
		enabledRoot := GetConfPath(kind + "s-enabled")
		availableRoot := GetConfPath(kind + "s-available")
		enabledPath := GetConfSymlinkPath(filepath.Join(enabledRoot, relativePath))
		var enabledCandidates []string
		switch kind {
		case "site":
			enabledCandidates = []string{
				enabledPath,
				filepath.Join(enabledRoot, relativePath),
				GetConfSymlinkPath(filepath.Join(enabledRoot, relativePath+maintenanceSuffix)),
				filepath.Join(enabledRoot, relativePath+maintenanceSuffix),
			}
		case "stream":
			enabledCandidates = []string{
				enabledPath,
				filepath.Join(enabledRoot, relativePath),
			}
		}
		selectedEnabledPath := ""
		for _, cand := range enabledCandidates {
			if helper.FileExists(cand) {
				selectedEnabledPath = cand
				break
			}
		}
		if selectedEnabledPath == "" {
			return "", nil
		}

		srcPath := selectedEnabledPath
		if fi, lErr := os.Lstat(selectedEnabledPath); lErr == nil && (fi.Mode()&os.ModeSymlink) != 0 {
			if target, rErr := os.Readlink(selectedEnabledPath); rErr == nil {
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(selectedEnabledPath), target)
				}
				srcPath = target
			}
		}
		if !helper.FileExists(srcPath) {
			srcPath = filepath.Join(availableRoot, relativePath)
		}
		content, rErr := os.ReadFile(srcPath)
		if rErr != nil {
			return "", fmt.Errorf("read %s content %s: %w", kind, srcPath, rErr)
		}

		rewritten, rErr := builder.rewriteConfigContent(string(content), srcPath)
		if rErr != nil {
			return "", fmt.Errorf("rewrite sandbox %s: %w", kind, rErr)
		}

		destName, rErr := filepath.Rel(enabledRoot, enabledPath)
		if rErr != nil || isPathOutsideBase(destName) {
			return "", fmt.Errorf("resolve sandbox %s destination %s: %w", kind, enabledPath, rErr)
		}

		destDir := filepath.Join(builder.sandboxDir, kind+"s-enabled")
		destPath := filepath.Join(destDir, destName)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return "", fmt.Errorf("create sandbox %s destination: %w", kind, err)
		}
		if err := ensureSandboxDestination(builder.sandboxDir, destPath); err != nil {
			return "", err
		}
		if err := os.WriteFile(destPath, []byte(rewritten), 0644); err != nil {
			return "", fmt.Errorf("write sandbox %s: %w", kind, err)
		}
		return destName, nil
	}

	seenSites := make(map[string]struct{}, len(sitePaths))
	for _, sp := range sitePaths {
		relativePath, rErr := relativeNamespaceConfigPath(sp, GetConfPath("sites-available"))
		if rErr != nil {
			return nil, nil, rErr
		}
		if written, wErr := readSourceAndWrite("site", relativePath); wErr != nil {
			return nil, nil, wErr
		} else if written != "" {
			if _, exists := seenSites[written]; exists {
				continue
			}
			seenSites[written] = struct{}{}
			siteFiles = append(siteFiles, written)
		}
	}

	seenStreams := make(map[string]struct{}, len(streamPaths))
	for _, st := range streamPaths {
		relativePath, rErr := relativeNamespaceConfigPath(st, GetConfPath("streams-available"))
		if rErr != nil {
			return nil, nil, rErr
		}
		if written, wErr := readSourceAndWrite("stream", relativePath); wErr != nil {
			return nil, nil, wErr
		} else if written != "" {
			if _, exists := seenStreams[written]; exists {
				continue
			}
			seenStreams[written] = struct{}{}
			streamFiles = append(streamFiles, written)
		}
	}

	return siteFiles, streamFiles, nil
}

func relativeNamespaceConfigPath(path, availableRoot string) (string, error) {
	cleanPath := filepath.Clean(path)
	relativePath, err := filepath.Rel(availableRoot, cleanPath)
	if err != nil || isPathOutsideBase(relativePath) {
		return "", &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("namespace config path %s is outside %s", path, availableRoot),
		}
	}
	return relativePath, nil
}

func isPathOutsideBase(relativePath string) bool {
	return relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath)
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
	tokens, err := tokenizeNginxConfig(content)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	lastWrite := 0
	statementStart := 0
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if i == statementStart && strings.EqualFold(token.value, "include") {
			if i+2 >= len(tokens) || tokens[i+2].value != ";" {
				return "", &SandboxBuildError{
					Category: ErrorCategorySyntaxError,
					Message:  "include directive must contain exactly one path argument",
				}
			}

			replacement, rewriteErr := b.rewriteIncludeDirective(tokens[i+1].value, filepath.Dir(sourcePath))
			if rewriteErr != nil {
				return "", rewriteErr
			}
			result.WriteString(content[lastWrite:token.start])
			result.WriteString(replacement)
			lastWrite = tokens[i+2].end
			i += 2
			statementStart = i + 1
			continue
		}

		switch token.value {
		case ";", "{", "}":
			statementStart = i + 1
		}
	}
	result.WriteString(content[lastWrite:])
	return result.String(), nil
}

func (b *sandboxBuilder) rewriteIncludeLine(line string, baseDir string) (string, error) {
	return b.rewriteConfigContent(line, filepath.Join(baseDir, "__include_line__.conf"))
}

func (b *sandboxBuilder) resolveIncludePath(includePath string, baseDir string) (string, []string, error) {
	if filepath.IsAbs(includePath) {
		matches, isPattern, err := matchIncludePattern(includePath)
		if err != nil {
			return "", nil, err
		}
		if len(matches) == 0 && !isPattern {
			return "", nil, newSandboxIncludeError(baseDir, includePath)
		}
		return includePath, matches, nil
	}

	candidates := uniqueSandboxCandidates(
		filepath.Clean(filepath.Join(b.confBase, includePath)),
		filepath.Clean(filepath.Join(baseDir, includePath)),
	)

	for _, candidate := range candidates {
		matches, isPattern, err := matchIncludePattern(candidate)
		if err != nil {
			return "", nil, err
		}
		if len(matches) > 0 || isPattern {
			return candidate, matches, nil
		}
	}

	if len(candidates) == 0 {
		return includePath, nil, nil
	}

	return "", nil, newSandboxIncludeError(baseDir, includePath)
}

func (b *sandboxBuilder) rewriteIncludeDirective(includePath, baseDir string) (string, error) {
	resolvedPath, matchedFiles, err := b.resolveIncludePath(includePath, baseDir)
	if err != nil {
		return "", err
	}

	if b.enabledDirectoryKind(resolvedPath) != "" {
		return "", nil
	}

	for _, matchedFile := range matchedFiles {
		if err := b.mirrorDependency(matchedFile); err != nil {
			return "", err
		}
	}

	sandboxPath, err := b.sandboxPathForSource(resolvedPath)
	if err != nil {
		return "", err
	}
	return "include " + quoteNginxPath(sandboxPath) + ";", nil
}

func (b *sandboxBuilder) enabledDirectoryKind(path string) string {
	for _, kind := range []string{"sites-enabled", "streams-enabled"} {
		if isPathWithin(filepath.Join(b.confBase, kind), path) {
			return kind
		}
	}
	return ""
}

func isPathWithin(root, path string) bool {
	relativePath, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && !isPathOutsideBase(relativePath)
}

func (b *sandboxBuilder) sandboxPathForSource(sourcePath string) (string, error) {
	if helper.IsUnderDirectory(sourcePath, b.confBase) {
		relativePath, err := filepath.Rel(b.confBase, sourcePath)
		if err != nil {
			return "", &SandboxBuildError{
				Category: ErrorCategorySandboxBuildError,
				Message:  fmt.Sprintf("failed to map sandbox path %s: %v", sourcePath, err),
			}
		}
		return filepath.Join(b.sandboxDir, relativePath), nil
	}

	directory := filepath.Dir(sourcePath)
	hash := sha256.Sum256([]byte(directory))
	return filepath.Join(b.sandboxDir, "__external", fmt.Sprintf("%x", hash[:8]), filepath.Base(sourcePath)), nil
}

func (b *sandboxBuilder) mirrorDependency(sourcePath string) error {
	sourcePath = filepath.Clean(sourcePath)
	if b.enabledDirectoryKind(sourcePath) != "" {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("refusing to mirror enabled configuration %s", sourcePath),
		}
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

	destPath, err := b.sandboxPathForSource(sourcePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("failed to create sandbox dependency dir for %s: %v", sourcePath, err),
		}
	}
	if err := ensureSandboxDestination(b.sandboxDir, destPath); err != nil {
		return err
	}

	if err := os.WriteFile(destPath, []byte(rewritten), 0644); err != nil {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("failed to write sandbox dependency %s: %v", sourcePath, err),
		}
	}

	return nil
}

func matchIncludePattern(pattern string) ([]string, bool, error) {
	isPattern := strings.ContainsAny(pattern, "*?[")
	if isPattern {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, true, &SandboxBuildError{
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

		return existing, true, nil
	}

	info, err := os.Stat(pattern)
	if err != nil || info.IsDir() {
		return nil, false, nil
	}

	return []string{pattern}, false, nil
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

// copyConfigBaseExceptSitesStreams copies readable regular configuration entries into the sandbox.
// Directory symlinks are materialized so later writes can never escape through a symlinked parent.
func copyConfigBaseExceptSitesStreams(sandboxDir string) error {
	activeDirectories := make(map[string]bool)
	return copySandboxConfigEntry(GetConfPath(), sandboxDir, sandboxDir, GetConfEntryPath(), activeDirectories, true)
}

func copySandboxConfigEntry(sourcePath, destPath, sandboxDir, entryPath string, activeDirectories map[string]bool, isRoot bool) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			logger.Debugf("Skipping unavailable sandbox config entry %s: %v", sourcePath, err)
			return nil
		}
		return err
	}

	if !isRoot {
		if filepath.Clean(sourcePath) == filepath.Clean(entryPath) {
			return nil
		}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := filepath.EvalSymlinks(sourcePath)
		if readErr != nil {
			logger.Debugf("Skipping dangling sandbox config symlink %s: %v", sourcePath, readErr)
			return nil
		}
		targetInfo, statErr := os.Stat(target)
		if statErr != nil {
			logger.Debugf("Skipping unavailable sandbox config symlink target %s: %v", sourcePath, statErr)
			return nil
		}
		base := filepath.Base(sourcePath)
		if targetInfo.IsDir() && (strings.HasPrefix(base, "sites-") || strings.HasPrefix(base, "streams-")) {
			return nil
		}
		return copySandboxConfigEntry(target, destPath, sandboxDir, entryPath, activeDirectories, false)
	}

	if info.IsDir() {
		base := filepath.Base(sourcePath)
		if !isRoot && (strings.HasPrefix(base, "sites-") || strings.HasPrefix(base, "streams-")) {
			return nil
		}
		canonicalPath, evalErr := filepath.EvalSymlinks(sourcePath)
		if evalErr != nil {
			canonicalPath = filepath.Clean(sourcePath)
		}
		if activeDirectories[canonicalPath] {
			logger.Debugf("Skipping recursive sandbox config directory %s", sourcePath)
			return nil
		}
		activeDirectories[canonicalPath] = true
		defer delete(activeDirectories, canonicalPath)

		if err := os.MkdirAll(destPath, 0755); err != nil {
			if os.IsPermission(err) || os.IsNotExist(err) {
				logger.Debugf("Skipping unavailable sandbox destination %s: %v", destPath, err)
				return nil
			}
			return err
		}
		entries, readErr := os.ReadDir(sourcePath)
		if readErr != nil {
			if os.IsPermission(readErr) || os.IsNotExist(readErr) {
				logger.Debugf("Skipping unreadable sandbox config directory %s: %v", sourcePath, readErr)
				return nil
			}
			return readErr
		}
		for _, entry := range entries {
			if err := copySandboxConfigEntry(
				filepath.Join(sourcePath, entry.Name()),
				filepath.Join(destPath, entry.Name()),
				sandboxDir,
				entryPath,
				activeDirectories,
				false,
			); err != nil {
				return err
			}
		}
		return nil
	}

	if !info.Mode().IsRegular() {
		logger.Debugf("Skipping non-regular sandbox config entry %s", sourcePath)
		return nil
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsPermission(err) || os.IsNotExist(err) {
			logger.Debugf("Skipping unreadable sandbox config file %s: %v", sourcePath, err)
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	if err := ensureSandboxDestination(sandboxDir, destPath); err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}

func ensureSandboxDestination(sandboxDir, destPath string) error {
	relativePath, err := filepath.Rel(sandboxDir, destPath)
	if err != nil || isPathOutsideBase(relativePath) {
		return &SandboxBuildError{
			Category: ErrorCategorySandboxBuildError,
			Message:  fmt.Sprintf("sandbox destination escapes sandbox directory: %s", destPath),
		}
	}

	current := filepath.Clean(sandboxDir)
	for _, component := range strings.Split(filepath.Clean(relativePath), string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, lstatErr := os.Lstat(current)
		if os.IsNotExist(lstatErr) {
			continue
		}
		if lstatErr != nil {
			return lstatErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &SandboxBuildError{
				Category: ErrorCategorySandboxBuildError,
				Message:  fmt.Sprintf("refusing to write through sandbox symlink %s", current),
			}
		}
	}
	return nil
}

func quoteNginxPath(path string) string {
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

type nginxConfigToken struct {
	value string
	start int
	end   int
}

func tokenizeNginxConfig(content string) ([]nginxConfigToken, error) {
	tokens := make([]nginxConfigToken, 0, len(content)/8)
	for index := 0; index < len(content); {
		switch content[index] {
		case ' ', '\t', '\r', '\n':
			index++
			continue
		case '#':
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		case '{', '}', ';':
			tokens = append(tokens, nginxConfigToken{value: content[index : index+1], start: index, end: index + 1})
			index++
			continue
		case '\'', '"':
			start := index
			quote := content[index]
			index++
			var value strings.Builder
			closed := false
			for index < len(content) {
				if content[index] == '\\' && index+1 < len(content) {
					value.WriteByte(content[index+1])
					index += 2
					continue
				}
				if content[index] == quote {
					index++
					closed = true
					break
				}
				value.WriteByte(content[index])
				index++
			}
			if !closed {
				return nil, newSandboxStructureError("unterminated quoted string in nginx configuration")
			}
			tokens = append(tokens, nginxConfigToken{value: value.String(), start: start, end: index})
			continue
		}

		start := index
		var value strings.Builder
		for index < len(content) {
			character := content[index]
			if character == '\\' && index+1 < len(content) {
				value.WriteByte(content[index+1])
				index += 2
				continue
			}
			if strings.ContainsRune(" \t\r\n#{};", rune(character)) {
				break
			}
			value.WriteByte(character)
			index++
		}
		if index == start {
			index++
			continue
		}
		tokens = append(tokens, nginxConfigToken{value: value.String(), start: start, end: index})
	}
	return tokens, nil
}

func newSandboxStructureError(message string) error {
	return &SandboxBuildError{
		Category: ErrorCategorySandboxBuildError,
		Message:  message,
	}
}
