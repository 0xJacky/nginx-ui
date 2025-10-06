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
	PeriodicScanInterval    time.Duration
	InitialScanTimeout      time.Duration
	ScanTimeoutGrace        time.Duration
	FileEventDebounce       time.Duration
	MaxFileSize             int64
	CallbackTimeout         time.Duration
	PostCallbackTimeout     time.Duration
	ShutdownTimeout         time.Duration
	ForceCleanupTimeout     time.Duration
	InitialScanWaitTimeout  time.Duration
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
		}
	})
	return excludedDirs
}

// shouldSkipPath checks if a path should be skipped during scanning or watching
func shouldSkipPath(path string) bool {
	for _, excludedDir := range getExcludedDirs() {
		if excludedDir != "" && strings.HasPrefix(path, excludedDir) {
			return true
		}
	}
	return false
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
			// Skip excluded directories (ssl, cache, logs, temp, etc.)
			if shouldSkipPath(path) {
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

	// Add new directories to watch
	if event.Has(fsnotify.Create) {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			if err := s.watcher.Add(event.Name); err != nil {
				logger.Error("Failed to add new directory to watcher:", event.Name, err)
			} else {
				logger.Debug("Added new directory to watcher:", event.Name)
			}
		}
	}

	// Handle file removal - need to trigger rescan to update indices
	if event.Has(fsnotify.Remove) {
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
		logger.Debugf("File skipped by shouldSkipPath: %s", filePath)
		return nil
	}

	// Get file info to check type and size
	fileInfo, err := os.Lstat(filePath) // Use Lstat to avoid following symlinks
	if err != nil {
		return err
	}

	// Skip directories
	if fileInfo.IsDir() {
		logger.Debugf("Skipping directory: %s", filePath)
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
			logger.Debugf("Skipping symlink to directory: %s", filePath)
			return nil
		}

		// Process symlinks to files, but use the target's info for size check
		fileInfo = targetInfo
		// logger.Debugf("Processing symlink to file: %s", filePath)
	}

	// Skip non-regular files (devices, pipes, sockets, etc.)
	if !fileInfo.Mode().IsRegular() {
		logger.Debugf("Skipping non-regular file: %s (mode: %s)", filePath, fileInfo.Mode())
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
				logger.Debugf("Skipping excluded directory: %s", fullPath)
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
			if err := s.scanSingleFileInternal(fullPath, true); err != nil {
				logger.Errorf("Failed to scan file %s: %v", fullPath, err)
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
