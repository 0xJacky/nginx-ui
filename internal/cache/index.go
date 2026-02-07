package cache

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
)

// ScanCallback is called during config scanning with file path and content
type ScanCallback func(configPath string, content []byte) error

// CallbackInfo stores callback function with its name for debugging
type CallbackInfo struct {
	Name     string
	Callback ScanCallback
}

// PostScanCallback is called after all scan callbacks are executed
type PostScanCallback func()

// ScanConfig holds scanner configuration
type ScanConfig struct {
	PeriodicScanInterval   time.Duration
	InitialScanTimeout     time.Duration
	ScanTimeoutGrace       time.Duration
	FileEventDebounce      time.Duration
	MaxFileSize            int64
	CallbackTimeout        time.Duration
	PostCallbackTimeout    time.Duration
	ShutdownTimeout        time.Duration
	ForceCleanupTimeout    time.Duration
	InitialScanWaitTimeout time.Duration
}

// DefaultScanConfig returns default configuration
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		PeriodicScanInterval:   5 * time.Minute,
		InitialScanTimeout:     15 * time.Second,
		ScanTimeoutGrace:       2 * time.Second,
		FileEventDebounce:      100 * time.Millisecond,
		MaxFileSize:            1024 * 1024, // 1MB
		CallbackTimeout:        5 * time.Second,
		PostCallbackTimeout:    10 * time.Second,
		ShutdownTimeout:        10 * time.Second,
		ForceCleanupTimeout:    3 * time.Second,
		InitialScanWaitTimeout: 30 * time.Second,
	}
}

var (
	postScanCallbacks      = make([]PostScanCallback, 0)
	postScanCallbacksMutex sync.RWMutex
	scanConfig             = DefaultScanConfig()
)

// runWithTimeout executes a function with timeout and panic protection
func runWithTimeout(fn func(), timeout time.Duration, name string) error {
	done := make(chan struct{})
	var panicErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = fmt.Errorf("panic: %v", r)
				logger.Errorf("%s panic: %v", name, r)
			}
			close(done)
		}()
		fn()
	}()

	select {
	case <-done:
		return panicErr
	case <-time.After(timeout):
		return fmt.Errorf("timeout after %v", timeout)
	}
}

// Scanner watches and scans nginx config files
type Scanner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	watcher    *fsnotify.Watcher
	scanTicker *time.Ticker
	scanning   bool
	scanMutex  sync.RWMutex
	wg         sync.WaitGroup // Track running goroutines
	debouncer  *fileEventDebouncer
}

// fileEventDebouncer prevents rapid repeated scans of the same file
type fileEventDebouncer struct {
	mu      sync.Mutex
	timers  map[string]*time.Timer
	stopped bool
}

func newFileEventDebouncer() *fileEventDebouncer {
	return &fileEventDebouncer{
		timers: make(map[string]*time.Timer),
	}
}

func (d *fileEventDebouncer) debounce(filePath string, delay time.Duration, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Don't create new timers if stopped
	if d.stopped {
		return
	}

	// Cancel existing timer if present
	if timer, exists := d.timers[filePath]; exists {
		timer.Stop()
	}

	// Create new timer
	d.timers[filePath] = time.AfterFunc(delay, func() {
		fn()
		// Cleanup
		d.mu.Lock()
		delete(d.timers, filePath)
		d.mu.Unlock()
	})
}

func (d *fileEventDebouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	// Stop and clear all pending timers
	for path, timer := range d.timers {
		timer.Stop()
		delete(d.timers, path)
	}
}

var (
	scanner            *Scanner
	scannerInitMutex   sync.Mutex
	scanCallbacks      = make([]CallbackInfo, 0)
	scanCallbacksMutex sync.RWMutex
	// Channel to signal when initial scan and all callbacks are completed
	initialScanComplete   chan struct{}
	initialScanOnce       sync.Once
	initialScanCompleteMu sync.Mutex // Protects initialScanComplete channel access
)

// InitScanner initializes the config scanner
func InitScanner(ctx context.Context) {
	if nginx.GetConfPath() == "" {
		logger.Error("Nginx config path is not set")
		return
	}

	// Force release any existing resources before initialization
	ForceReleaseResources()

	scanner := GetScanner()
	if err := scanner.Initialize(ctx); err != nil {
		logger.Error("Failed to initialize config scanner:", err)
		// On failure, force cleanup
		ForceReleaseResources()
	}
}

var (
	excludedDirs     []string
	excludedDirsOnce sync.Once
)

// getExcludedDirs returns cached list of excluded directories
func getExcludedDirs() []string {
	excludedDirsOnce.Do(func() {
		excludedDirs = []string{
			nginx.GetConfPath("ssl"),
			nginx.GetConfPath("cache"),
			nginx.GetConfPath("logs"),
			nginx.GetConfPath("temp"),
			nginx.GetConfPath("proxy_temp"),
			nginx.GetConfPath("client_body_temp"),
			nginx.GetConfPath("fastcgi_temp"),
			nginx.GetConfPath("uwsgi_temp"),
			nginx.GetConfPath("scgi_temp"),
			// Static asset directories - these can contain thousands of files
			// and should not trigger config scanning
			nginx.GetConfPath("html"),
			nginx.GetConfPath("www"),
			nginx.GetConfPath("static"),
			nginx.GetConfPath("assets"),
			nginx.GetConfPath("public"),
			nginx.GetConfPath("webroot"),
		}
	})
	return excludedDirs
}

// shouldSkipPath checks if a path should be skipped during scanning or watching
func shouldSkipPath(path string) bool {
	for _, excludedDir := range getExcludedDirs() {
		if excludedDir == "" {
			continue
		}
		// Check for exact match or match with path separator to avoid false positives
		// e.g., excludedDir="/etc/nginx/html" should match "/etc/nginx/html/file"
		// but NOT "/etc/nginx/html-configs/file"
		if path == excludedDir || strings.HasPrefix(path, excludedDir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// staticDirNames contains directory names that typically contain static assets and should not be watched.
// This single list is used for both Contains (with "/" suffix) and HasSuffix (with "/" prefix) checks.
var staticDirNames = []string{
	"dist",
	"build",
	"node_modules",
	"__pycache__",
	".git",
	"vendor",
	"assets",
	"static",
	"public",
	"media",
	"uploads",
	"images",
	"img",
	"css",
	"js",
	"fonts",
	"__macosx",
}

// configDirPatterns contains directory names that typically contain nginx config files.
// Used with path-separator boundaries to avoid false positives.
var configDirPatterns = []string{
	"sites-available", "sites-enabled",
	"streams-available", "streams-enabled",
	"conf.d", "snippets", "modules-enabled",
}

// configFilePatterns contains common nginx config file names without extension.
var configFilePatterns = []string{
	"nginx.conf",
	"mime.types",
	"fastcgi_params",
	"fastcgi.conf",
	"scgi_params",
	"uwsgi_params",
	"koi-utf",
	"koi-win",
	"win-utf",
	"proxy_params",
}

// nonConfigExtensions contains file extensions that are definitely not config files.
// This is a safeguard for files that might be in the root nginx directory.
var nonConfigExtensions = map[string]bool{
	// Web assets
	".html": true, ".htm": true, ".css": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".json": true, ".xml": true, ".svg": true, ".map": true, ".woff": true, ".woff2": true,
	".ttf": true, ".eot": true, ".otf": true,
	// Images
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".webp": true,
	".bmp": true, ".tiff": true, ".avif": true,
	// Archives
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".rar": true, ".7z": true,
	// Documents
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	// Media
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true, ".webm": true,
	".ogg": true, ".wav": true,
	// Other binaries
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	// Source code (not nginx config)
	".py": true, ".rb": true, ".php": true, ".java": true, ".go": true, ".rs": true, ".c": true, ".cpp": true,
	".h": true, ".hpp": true, ".sh": true, ".bat": true, ".ps1": true,
	// Data files
	".db": true, ".sqlite": true, ".sql": true, ".csv": true, ".yml": true, ".yaml": true, ".toml": true,
	".md": true, ".txt": true, ".log": true, ".lock": true,
}

// shouldWatchDirectory checks if a directory should be watched for config file changes
// This prevents watching static asset directories that can contain thousands of files
func shouldWatchDirectory(dirPath string) bool {
	// Check if directory matches excluded paths
	if shouldSkipPath(dirPath) {
		return false
	}

	// Get the path relative to the nginx config root to avoid matching ancestor directories
	// e.g., if config root is /opt/vendor/nginx/conf, we don't want to match "/vendor/"
	// in the ancestor portion of the path
	configRoot := nginx.GetConfPath()
	relativePath := dirPath
	if strings.HasPrefix(dirPath, configRoot) {
		relativePath = strings.TrimPrefix(dirPath, configRoot)
	}
	lowerRelativePath := strings.ToLower(relativePath)

	// Check static directory patterns against the relative path only
	// This ensures patterns like "/vendor/" only match directories within the config tree,
	// not ancestor directories in the config root path itself
	sep := string(filepath.Separator)
	for _, name := range staticDirNames {
		// Check if pattern appears in the middle of path (with slashes on both sides)
		if strings.Contains(lowerRelativePath, sep+name+sep) {
			return false
		}
		// Check if path ends with this directory name (with leading slash)
		if strings.HasSuffix(lowerRelativePath, sep+name) {
			return false
		}
	}

	// All directories that pass the static directory filter should be watched
	// This includes known config directories (sites-available, conf.d, etc.) and any other
	// directories that might contain nginx config files
	return true
}

// isConfigFilePath checks if a file path appears to be a nginx configuration file
// This filters out static assets, binary files, and other non-config files
func isConfigFilePath(filePath string) bool {
	// Get the file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	baseName := strings.ToLower(filepath.Base(filePath))

	// Use relative path to avoid matching ancestor directories in the config root
	// e.g., if config root is /srv/conf.d/nginx/, we don't want to match "conf.d"
	// in the ancestor portion of the path
	configRoot := nginx.GetConfPath()
	relativePath := filePath
	if strings.HasPrefix(filePath, configRoot) {
		relativePath = strings.TrimPrefix(filePath, configRoot)
	}
	lowerRelativePath := strings.ToLower(relativePath)
	sep := string(filepath.Separator)

	// Check static directory patterns FIRST against the relative path
	// This must come before config dir check to prevent files in
	// /etc/nginx/sites-enabled/project/dist/bundle.js from being treated as config
	for _, name := range staticDirNames {
		// Check if pattern appears in the path (with slashes on both sides)
		if strings.Contains(lowerRelativePath, sep+name+sep) {
			return false
		}
	}

	// Check for common nginx config file patterns using relative path with path-separator boundaries
	// Files in sites-available/sites-enabled/streams-available/streams-enabled/conf.d
	// are typically config files. Use separator-bounded matching to avoid false positives
	// like "myconf.db" matching "conf.d" across the name/extension boundary
	for _, pattern := range configDirPatterns {
		// Check if pattern appears in the path (with slashes on both sides)
		// This covers both middle-of-path and start-of-path cases since relativePath
		// always starts with a separator after TrimPrefix from a Clean'd configRoot
		if strings.Contains(lowerRelativePath, sep+pattern+sep) {
			return true
		}
	}

	// Files with .conf extension are config files
	if ext == ".conf" {
		return true
	}

	// Common nginx config file patterns without extension
	// nginx.conf, mime.types, fastcgi_params, etc.
	for _, pattern := range configFilePatterns {
		if baseName == pattern {
			return true
		}
	}

	// Exclude common static asset extensions that are definitely not config files
	// This is a safeguard for files that might be in the root nginx directory
	if nonConfigExtensions[ext] {
		return false
	}

	// For files without recognized extensions that passed all filters,
	// we conservatively treat them as potential config files
	// (this allows sites-available/mysite type files)
	return true
}

// GetScanner returns the singleton scanner instance
func GetScanner() *Scanner {
	scannerInitMutex.Lock()
	defer scannerInitMutex.Unlock()

	if scanner == nil {
		scanner = &Scanner{
			debouncer: newFileEventDebouncer(),
		}
	}
	return scanner
}

// RegisterCallback adds a named callback to be executed during scans
func RegisterCallback(name string, callback ScanCallback) {
	scanCallbacksMutex.Lock()
	defer scanCallbacksMutex.Unlock()

	scanCallbacks = append(scanCallbacks, CallbackInfo{
		Name:     name,
		Callback: callback,
	})
}

// RegisterPostScanCallback adds a callback to be executed after all scan callbacks complete
func RegisterPostScanCallback(callback PostScanCallback) {
	postScanCallbacksMutex.Lock()
	defer postScanCallbacksMutex.Unlock()

	postScanCallbacks = append(postScanCallbacks, callback)
}

// Initialize sets up the scanner and starts watching
func (s *Scanner) Initialize(ctx context.Context) error {
	// Initialize the completion channel for this scan cycle with lock protection
	initialScanCompleteMu.Lock()
	initialScanComplete = make(chan struct{})
	initialScanOnce = sync.Once{} // Reset for this initialization
	initialScanCompleteMu.Unlock()

	// Create cancellable context for this scanner instance
	s.ctx, s.cancel = context.WithCancel(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	s.watcher = watcher

	// Watch all directories recursively first (this is faster than scanning)
	if err := s.watchAllDirectories(); err != nil {
		return err
	}

	// Start background processes
	s.wg.Go(func() {
		s.watchForChanges()
	})

	s.wg.Go(func() {
		s.periodicScan()
	})

	// Perform initial scan asynchronously to avoid blocking boot process
	s.wg.Go(func() {
		s.initialScanAsync(ctx)
	})

	return nil
}

// watchAllDirectories recursively adds all directories under nginx config path to watcher
func (s *Scanner) watchAllDirectories() error {
	root := nginx.GetConfPath()

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip excluded directories (ssl, cache, logs, temp, static assets, etc.)
			if shouldSkipPath(path) {
				logger.Debug("Skipping excluded directory from watcher:", path)
				return filepath.SkipDir
			}

			// Skip directories that shouldn't be watched (static assets, etc.)
			if !shouldWatchDirectory(path) {
				logger.Debug("Skipping non-config directory from watcher:", path)
				return filepath.SkipDir
			}

			// Resolve symlinks to get the actual directory path to watch
			actualPath := path
			if d.Type()&os.ModeSymlink != 0 {
				// This is a symlink, resolve it to get the target path
				if resolvedPath, err := filepath.EvalSymlinks(path); err == nil {
					actualPath = resolvedPath
					logger.Debug("Resolved symlink for watching:", path, "->", actualPath)
				} else {
					logger.Debug("Failed to resolve symlink, skipping:", path, err)
					return filepath.SkipDir
				}
			}

			if err := s.watcher.Add(actualPath); err != nil {
				logger.Error("Failed to watch directory:", actualPath, err)
				return err
			}
		}
		return nil
	})
}

// periodicScan runs periodic scans
func (s *Scanner) periodicScan() {
	s.scanTicker = time.NewTicker(scanConfig.PeriodicScanInterval)
	defer s.scanTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Debug("periodicScan: context cancelled, exiting")
			return
		case <-s.scanTicker.C:
			if err := s.ScanAllConfigs(); err != nil {
				logger.Error("Periodic scan failed:", err)
			}
		}
	}
}

// initialScanAsync performs the initial config scan asynchronously
func (s *Scanner) initialScanAsync(ctx context.Context) {
	// Always use the provided context, not the scanner's internal context
	// This ensures we use the fresh boot context, not a potentially cancelled old context
	logger.Debugf("Initial scan starting with context: cancelled=%v", ctx.Err() != nil)

	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		logger.Warn("Initial scan cancelled before starting - context already done")
		// Signal completion even when cancelled early so waiting services don't hang
		initialScanOnce.Do(func() {
			logger.Warn("Initial config scan cancelled early - signaling completion")
			close(initialScanComplete)
		})
		return
	default:
	}

	logger.Debug("Starting initial config scan...")
	logger.Debugf("Config path: %s", nginx.GetConfPath())

	// Perform the scan with the fresh context (not scanner's internal context)
	if err := s.scanAllConfigsWithContext(ctx); err != nil {
		// Only log error if it's not due to context cancellation
		if ctx.Err() == nil {
			logger.Errorf("Initial config scan failed: %v", err)
		} else {
			logger.Debugf("Initial config scan cancelled due to context: %v", ctx.Err())
		}
		// Signal completion even on error so waiting services don't hang
		initialScanOnce.Do(func() {
			logger.Warn("Initial config scan completed with error - signaling completion anyway")
			close(initialScanComplete)
		})
	} else {
		// Signal that initial scan is complete - this allows other services to proceed
		// that depend on the scan callbacks to have been processed
		initialScanOnce.Do(func() {
			logger.Debug("Initial config scan and callbacks completed - signaling completion")
			close(initialScanComplete)
		})
	}
}

// scanAllConfigsWithContext scans all nginx configuration files with context support
func (s *Scanner) scanAllConfigsWithContext(ctx context.Context) error {
	s.setScanningState(true)
	defer s.setScanningState(false)

	root := nginx.GetConfPath()
	logger.Debugf("Scanning config directory: %s", root)

	// Create a timeout context for the scan operation
	scanCtx, scanCancel := context.WithTimeout(ctx, scanConfig.InitialScanTimeout)
	defer scanCancel()

	// Scan all files in the config directory and subdirectories
	logger.Debug("Starting filepath.WalkDir scanning...")

	// Use a channel to communicate scan results
	type scanResult struct {
		err       error
		fileCount int
		dirCount  int
	}
	resultChan := make(chan scanResult, 1)

	// Run custom directory traversal in a goroutine to avoid WalkDir blocking issues
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("Scan goroutine panic: %v", r)
				resultChan <- scanResult{err: fmt.Errorf("panic during scan: %v", r)}
			}
		}()

		fileCount := 0
		dirCount := 0

		// Use custom recursive traversal instead of filepath.WalkDir
		walkErr := s.scanDirectoryRecursive(scanCtx, root, &fileCount, &dirCount)

		// Send result through channel
		resultChan <- scanResult{
			err:       walkErr,
			fileCount: fileCount,
			dirCount:  dirCount,
		}
	}()

	// Wait for scan to complete or timeout
	var scanErr error
	select {
	case result := <-resultChan:
		logger.Debugf("Scan completed successfully: dirs=%d, files=%d, error=%v",
			result.dirCount, result.fileCount, result.err)
		scanErr = result.err
	case <-scanCtx.Done():
		logger.Warnf("Scan timed out after 25 seconds - cancelling")
		scanCancel()
		// Wait a bit more for cleanup
		select {
		case result := <-resultChan:
			logger.Debugf("Scan completed after timeout: dirs=%d, files=%d, error=%v",
				result.dirCount, result.fileCount, result.err)
			scanErr = result.err
		case <-time.After(scanConfig.ScanTimeoutGrace):
			logger.Warn("Scan failed to complete even after timeout - forcing return")
			scanErr = ctx.Err()
		}
	}

	// Trigger post-scan callbacks once after all files are scanned
	if scanErr == nil {
		s.executePostScanCallbacks()
	}

	return scanErr
}

// watchForChanges handles file system events
func (s *Scanner) watchForChanges() {
	for {
		select {
		case <-s.ctx.Done():
			logger.Debug("watchForChanges: context cancelled, exiting")
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				logger.Debug("watchForChanges: events channel closed, exiting")
				return
			}
			s.handleFileEvent(event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
				logger.Debug("watchForChanges: errors channel closed, exiting")
				return
			}
			logger.Error("Watcher error:", err)
		}
	}
}

// handleFileEvent processes individual file system events
func (s *Scanner) handleFileEvent(event fsnotify.Event) {
	// Only handle relevant events
	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) &&
		!event.Has(fsnotify.Rename) && !event.Has(fsnotify.Remove) {
		return
	}

	// Skip excluded directories (ssl, cache, etc.)
	if shouldSkipPath(event.Name) {
		return
	}

	// Add new directories to watch (but only if they could contain config files)
	if event.Has(fsnotify.Create) {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			// Skip adding directories that are clearly static asset directories
			if shouldWatchDirectory(event.Name) {
				if err := s.watcher.Add(event.Name); err != nil {
					logger.Error("Failed to add new directory to watcher:", event.Name, err)
				} else {
					logger.Debug("Added new directory to watcher:", event.Name)
				}
			} else {
				logger.Debug("Skipping non-config directory from watcher:", event.Name)
			}
		}
	}

	// Handle file removal - need to trigger rescan to update indices
	if event.Has(fsnotify.Remove) {
		// Only process config file removals
		if !isConfigFilePath(event.Name) {
			return
		}
		logger.Debug("Config removed:", event.Name)
		// Trigger callbacks with empty content to allow them to clean up their indices
		// Don't skip post-scan for single file events (manual operations)
		s.executeCallbacks(event.Name, []byte{}, false)
		return
	}

	// Use Lstat to get symlink info without following it
	fi, err := os.Lstat(event.Name)
	if err != nil {
		return
	}

	// If it's a symlink, we need to check what it points to
	var targetIsDir bool
	if fi.Mode()&os.ModeSymlink != 0 {
		// For symlinks, check the target
		targetFi, err := os.Stat(event.Name)
		if err != nil {
			logger.Debug("Symlink target not accessible:", event.Name, err)
			return
		}
		targetIsDir = targetFi.IsDir()
		logger.Debug("Symlink changed:", event.Name, "-> target is dir:", targetIsDir)
	} else {
		targetIsDir = fi.IsDir()
	}

	if targetIsDir {
		logger.Debug("Directory changed:", event.Name)
	} else {
		// Skip non-config files to avoid I/O overload from static assets
		if !isConfigFilePath(event.Name) {
			return
		}
		logger.Debug("File changed:", event.Name)
		// Use debouncer to avoid rapid repeated scans
		s.debouncer.debounce(event.Name, scanConfig.FileEventDebounce, func() {
			s.scanSingleFile(event.Name)
		})
	}
}

// scanSingleFile scans a single config file without recursion
// skipPostScan: if true, skip post-scan callbacks (used during batch scans)
func (s *Scanner) scanSingleFile(filePath string) error {
	return s.scanSingleFileInternal(filePath, false)
}

// scanSingleFileInternal is the internal implementation with post-scan control
func (s *Scanner) scanSingleFileInternal(filePath string, skipPostScan bool) error {
	s.setScanningState(true)
	defer s.setScanningState(false)

	// Check if path should be skipped
	if shouldSkipPath(filePath) {
		return nil
	}

	// Skip non-config files early to avoid unnecessary I/O
	if !isConfigFilePath(filePath) {
		return nil
	}

	// Get file info to check type and size
	fileInfo, err := os.Lstat(filePath) // Use Lstat to avoid following symlinks
	if err != nil {
		return err
	}

	// Skip directories
	if fileInfo.IsDir() {
		return nil
	}

	// Handle symlinks carefully
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		// Check what the symlink points to
		targetInfo, err := os.Stat(filePath)
		if err != nil {
			logger.Debugf("Skipping symlink with inaccessible target: %s (%v)", filePath, err)
			return nil
		}

		// Skip symlinks to directories
		if targetInfo.IsDir() {
			return nil
		}

		// Process symlinks to files, but use the target's info for size check
		fileInfo = targetInfo
	}

	// Skip non-regular files (devices, pipes, sockets, etc.)
	if !fileInfo.Mode().IsRegular() {
		return nil
	}

	// Skip files larger than max size before reading
	if fileInfo.Size() > scanConfig.MaxFileSize {
		logger.Debugf("Skipping large file: %s (size: %d bytes)", filePath, fileInfo.Size())
		return nil
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		logger.Errorf("os.ReadFile failed for %s: %v", filePath, err)
		return err
	}

	// Execute callbacks
	s.executeCallbacks(filePath, content, skipPostScan)

	return nil
}

// setScanningState updates the scanning state and publishes events
func (s *Scanner) setScanningState(scanning bool) {
	s.scanMutex.Lock()
	defer s.scanMutex.Unlock()

	if s.scanning != scanning {
		s.scanning = scanning
		event.Publish(event.Event{
			Type: event.TypeIndexScanning,
			Data: scanning,
		})
	}
}

// executeCallbacks runs all registered callbacks
func (s *Scanner) executeCallbacks(filePath string, content []byte, skipPostScan bool) {
	scanCallbacksMutex.RLock()
	callbacksCopy := make([]CallbackInfo, len(scanCallbacks))
	copy(callbacksCopy, scanCallbacks)
	scanCallbacksMutex.RUnlock()

	for i, callbackInfo := range callbacksCopy {
		// Add timeout protection for each callback
		done := make(chan error, 1)
		go func() {
			done <- callbackInfo.Callback(filePath, content)
		}()

		select {
		case err := <-done:
			if err != nil {
				logger.Errorf("Callback error for %s in '%s': %v", filePath, callbackInfo.Name, err)
			}
		case <-time.After(scanConfig.CallbackTimeout):
			logger.Errorf("Callback [%d/%d] '%s' timed out after %v for: %s", i+1, len(callbacksCopy), callbackInfo.Name, scanConfig.CallbackTimeout, filePath)
			// Continue with next callback instead of blocking forever
		}
	}

	// Execute post-scan callbacks only if not skipped (used for batch scans)
	if !skipPostScan {
		s.executePostScanCallbacks()
	}
}

// executePostScanCallbacks runs all registered post-scan callbacks
func (s *Scanner) executePostScanCallbacks() {
	postScanCallbacksMutex.RLock()
	postCallbacksCopy := make([]PostScanCallback, len(postScanCallbacks))
	copy(postCallbacksCopy, postScanCallbacks)
	postScanCallbacksMutex.RUnlock()

	for i, callback := range postCallbacksCopy {
		name := fmt.Sprintf("Post-scan callback [%d/%d]", i+1, len(postCallbacksCopy))
		if err := runWithTimeout(callback, scanConfig.PostCallbackTimeout, name); err != nil {
			logger.Errorf("%s error: %v", name, err)
		}
	}
}

// ScanAllConfigs scans all nginx configuration files
func (s *Scanner) ScanAllConfigs() error {
	s.setScanningState(true)
	defer s.setScanningState(false)

	root := nginx.GetConfPath()
	fileCount := 0
	dirCount := 0

	// Use the unified recursive scan logic with no timeout
	err := s.scanDirectoryRecursive(context.Background(), root, &fileCount, &dirCount)

	logger.Debugf("Scan completed: %d directories, %d files processed", dirCount, fileCount)

	// Trigger post-scan callbacks once after all files are scanned
	if err == nil {
		s.executePostScanCallbacks()
	}

	return err
}

// scanDirectoryRecursive implements custom recursive directory traversal
// to avoid filepath.WalkDir blocking issues on restart
func (s *Scanner) scanDirectoryRecursive(ctx context.Context, root string, fileCount, dirCount *int) error {
	visited := make(map[string]bool)
	return s.scanDirectoryRecursiveInternal(ctx, root, fileCount, dirCount, visited)
}

// scanDirectoryRecursiveInternal is the internal implementation with symlink loop detection
func (s *Scanner) scanDirectoryRecursiveInternal(ctx context.Context, root string, fileCount, dirCount *int, visited map[string]bool) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Resolve symlinks and check for loops
	realPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		// If we can't resolve, use original path
		realPath = root
	}

	// Check if already visited (prevents symlink loops)
	if visited[realPath] {
		logger.Debugf("Skipping already visited path (symlink loop): %s -> %s", root, realPath)
		return nil
	}
	visited[realPath] = true

	// Read directory entries
	entries, err := os.ReadDir(root)
	if err != nil {
		logger.Errorf("Failed to read directory %s: %v", root, err)
		return err
	}

	// Process each entry
	for i, entry := range entries {
		// Check context cancellation periodically
		if i%10 == 0 {
			select {
			case <-ctx.Done():
				logger.Warnf("Scan cancelled while processing entries in: %s", root)
				return ctx.Err()
			default:
			}
		}

		fullPath := filepath.Join(root, entry.Name())

		entryType := entry.Type()

		isDir := entry.IsDir()

		if isDir {
			(*dirCount)++

			// Skip excluded directories
			if shouldSkipPath(fullPath) {
				continue
			}

			// Skip directories that shouldn't be scanned (static assets, etc.)
			if !shouldWatchDirectory(fullPath) {
				continue
			}

			// Recursively scan subdirectory - continue on error to scan other directories
			if err := s.scanDirectoryRecursiveInternal(ctx, fullPath, fileCount, dirCount, visited); err != nil {
				logger.Errorf("Failed to scan subdirectory %s: %v", fullPath, err)
				// Continue with other directories instead of failing completely
			}
		} else {
			(*fileCount)++

			// Handle symlinks
			if entryType&os.ModeSymlink != 0 {
				targetInfo, err := os.Stat(fullPath)
				if err == nil {
					if targetInfo.IsDir() {
						// Check if symlink directory should be scanned
						if !shouldWatchDirectory(fullPath) {
							continue
						}
						// Recursively scan symlink directory (with loop detection)
						if err := s.scanDirectoryRecursiveInternal(ctx, fullPath, fileCount, dirCount, visited); err != nil {
							logger.Errorf("Failed to scan symlink directory %s: %v", fullPath, err)
						}
						continue
					}
				} else {
					logger.Warnf("os.Stat failed for symlink %s: %v", fullPath, err)
				}
			}

			// Process regular files - skip post-scan during batch scan
			// scanSingleFileInternal already checks isConfigFilePath, but we skip early for efficiency
			if isConfigFilePath(fullPath) {
				if err := s.scanSingleFileInternal(fullPath, true); err != nil {
					logger.Errorf("Failed to scan file %s: %v", fullPath, err)
				}
			}
		}
	}

	return nil
}

// Shutdown cleans up scanner resources
func (s *Scanner) Shutdown() {
	logger.Info("Starting scanner shutdown...")

	// Cancel context to signal all goroutines to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Stop debouncer to prevent new scans
	if s.debouncer != nil {
		s.debouncer.stop()
	}

	// Close watcher first to stop file events
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}

	// Stop ticker
	if s.scanTicker != nil {
		s.scanTicker.Stop()
		s.scanTicker = nil
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All scanner goroutines completed successfully")
	case <-time.After(scanConfig.ShutdownTimeout):
		logger.Warn("Timeout waiting for scanner goroutines to complete")
	}

	// Clear the global scanner instance to force recreation on next use
	scannerInitMutex.Lock()
	scanner = nil
	// Reset initialization state for next restart
	scannerInitMutex.Unlock()

	logger.Info("Scanner shutdown completed and global instance cleared for recreation")
}

// IsScanningInProgress returns whether a scan is currently running
func IsScanningInProgress() bool {
	s := GetScanner()
	s.scanMutex.RLock()
	defer s.scanMutex.RUnlock()
	return s.scanning
}

// ForceReleaseResources performs aggressive cleanup of all file system resources
func ForceReleaseResources() {
	scannerInitMutex.Lock()
	defer scannerInitMutex.Unlock()

	logger.Info("Force releasing all scanner resources...")

	if scanner != nil {
		// Cancel context first to signal all goroutines
		if scanner.cancel != nil {
			logger.Info("Cancelling scanner context to stop all operations")
			scanner.cancel()
		}

		// Wait a brief moment for operations to respond to cancellation
		time.Sleep(200 * time.Millisecond)

		// Force close file system watcher - this should release all locks
		if scanner.watcher != nil {
			logger.Info("Forcefully closing file system watcher and releasing all file locks")
			if err := scanner.watcher.Close(); err != nil {
				logger.Errorf("Error force-closing watcher: %v", err)
			} else {
				logger.Info("File system watcher force-closed, locks should be released")
			}
			scanner.watcher = nil
		}

		// Stop ticker
		if scanner.scanTicker != nil {
			logger.Info("Stopping scan ticker")
			scanner.scanTicker.Stop()
			scanner.scanTicker = nil
		}

		// Wait for goroutines to complete with short timeout
		done := make(chan struct{})
		go func() {
			scanner.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("All scanner goroutines terminated successfully")
		case <-time.After(scanConfig.ForceCleanupTimeout):
			logger.Warn("Timeout waiting for scanner goroutines - proceeding with force cleanup")
		}

		scanner = nil
	}
}

// WaitForInitialScanComplete waits for the initial config scan and all callbacks to complete
// This is useful for services that depend on site indexing to be ready
func WaitForInitialScanComplete() {
	// Get channel reference with lock to avoid race
	initialScanCompleteMu.Lock()
	ch := initialScanComplete
	initialScanCompleteMu.Unlock()

	if ch == nil {
		logger.Debug("Initial scan completion channel not initialized, returning immediately")
		return
	}

	logger.Debug("Waiting for initial config scan to complete...")

	// Add timeout to prevent infinite waiting
	select {
	case <-ch:
		logger.Debug("Initial config scan completion confirmed")
	case <-time.After(scanConfig.InitialScanWaitTimeout):
		logger.Warn("Timeout waiting for initial config scan completion - proceeding anyway")
	}
}
