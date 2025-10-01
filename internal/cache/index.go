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

// Scanner watches and scans nginx config files
type Scanner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	watcher    *fsnotify.Watcher
	scanTicker *time.Ticker
	scanning   bool
	scanMutex  sync.RWMutex
	wg         sync.WaitGroup // Track running goroutines
}

var (
	scanner            *Scanner
	scannerInitMutex   sync.Mutex
	scanCallbacks      = make([]CallbackInfo, 0)
	scanCallbacksMutex sync.RWMutex
	// Channel to signal when initial scan and all callbacks are completed
	initialScanComplete chan struct{}
	initialScanOnce     sync.Once
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

// shouldSkipPath checks if a path should be skipped during scanning or watching
func shouldSkipPath(path string) bool {
	// Define directories to exclude from scanning/watching
	excludedDirs := []string{
		nginx.GetConfPath("ssl"),              // SSL certificates and keys
		nginx.GetConfPath("cache"),            // Nginx cache files
		nginx.GetConfPath("logs"),             // Log files directory
		nginx.GetConfPath("temp"),             // Temporary files directory
		nginx.GetConfPath("proxy_temp"),       // Proxy temporary files
		nginx.GetConfPath("client_body_temp"), // Client body temporary files
		nginx.GetConfPath("fastcgi_temp"),     // FastCGI temporary files
		nginx.GetConfPath("uwsgi_temp"),       // uWSGI temporary files
		nginx.GetConfPath("scgi_temp"),        // SCGI temporary files
	}

	// Check if path starts with any excluded directory
	for _, excludedDir := range excludedDirs {
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
		scanner = &Scanner{}
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

// Initialize sets up the scanner and starts watching
func (s *Scanner) Initialize(ctx context.Context) error {
	// Initialize the completion channel for this scan cycle
	initialScanComplete = make(chan struct{})
	initialScanOnce = sync.Once{} // Reset for this initialization

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

	// Start background processes with WaitGroup tracking
	s.wg.Go(func() {
		logger.Debug("Started cache watchForChanges goroutine")
		s.watchForChanges()
		logger.Info("Cache watchForChanges goroutine completed")
	})

	s.wg.Go(func() {
		logger.Debug("Started cache periodicScan goroutine")
		s.periodicScan()
		logger.Info("Cache periodicScan goroutine completed")
	})

	s.wg.Go(func() {
		logger.Debug("Started cache handleShutdown goroutine")
		s.handleShutdown()
		logger.Info("Cache handleShutdown goroutine completed")
	})

	// Perform initial scan asynchronously to avoid blocking boot process
	// Pass the context to ensure proper cancellation
	s.wg.Go(func() {
		logger.Debug("Started cache initialScanAsync goroutine")
		s.initialScanAsync(ctx)
		logger.Debug("Cache initialScanAsync goroutine completed")
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

// periodicScan runs periodic scans every 5 minutes
func (s *Scanner) periodicScan() {
	s.scanTicker = time.NewTicker(5 * time.Minute)
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

// handleShutdown listens for context cancellation and shuts down gracefully
func (s *Scanner) handleShutdown() {
	<-s.ctx.Done()
	logger.Debug("Shutting down Index Scanner")
	// Note: Don't call s.Shutdown() here as it would cause deadlock
	// Shutdown is called externally, this just handles cleanup
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
	scanCtx, scanCancel := context.WithTimeout(ctx, 15*time.Second)
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
	select {
	case result := <-resultChan:
		logger.Debugf("Scan completed successfully: dirs=%d, files=%d, error=%v",
			result.dirCount, result.fileCount, result.err)
		return result.err
	case <-scanCtx.Done():
		logger.Warnf("Scan timed out after 25 seconds - cancelling")
		scanCancel()
		// Wait a bit more for cleanup
		select {
		case result := <-resultChan:
			logger.Debugf("Scan completed after timeout: dirs=%d, files=%d, error=%v",
				result.dirCount, result.fileCount, result.err)
			return result.err
		case <-time.After(2 * time.Second):
			logger.Warn("Scan failed to complete even after timeout - forcing return")
			return ctx.Err()
		}
	}
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

	// Handle file changes
	if event.Has(fsnotify.Remove) {
		logger.Debug("Config removed:", event.Name)
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
		time.Sleep(100 * time.Millisecond) // Allow file write to complete
		s.scanSingleFile(event.Name)
	}
}

// scanSingleFile scans a single config file without recursion
func (s *Scanner) scanSingleFile(filePath string) error {
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

	// Skip files larger than 1MB before reading
	if fileInfo.Size() > 1024*1024 {
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
	s.executeCallbacks(filePath, content)

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
func (s *Scanner) executeCallbacks(filePath string, content []byte) {
	scanCallbacksMutex.RLock()
	defer scanCallbacksMutex.RUnlock()

	for i, callbackInfo := range scanCallbacks {
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
		case <-time.After(5 * time.Second):
			logger.Errorf("Callback [%d/%d] '%s' timed out after 5 seconds for: %s", i+1, len(scanCallbacks), callbackInfo.Name, filePath)
			// Continue with next callback instead of blocking forever
		}
	}
}

// ScanAllConfigs scans all nginx configuration files
func (s *Scanner) ScanAllConfigs() error {
	s.setScanningState(true)
	defer s.setScanningState(false)

	root := nginx.GetConfPath()

	// Scan all files in the config directory and subdirectories
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories (ssl, cache, logs, temp, etc.)
		if d.IsDir() && shouldSkipPath(path) {
			return filepath.SkipDir
		}

		// Handle symlinks to directories specially
		if d.Type()&os.ModeSymlink != 0 {
			if targetInfo, err := os.Stat(path); err == nil && targetInfo.IsDir() {
				// This is a symlink to a directory, we should traverse its contents
				// but not process the symlink itself as a file
				logger.Debug("Found symlink to directory, will traverse contents:", path)

				// Manually scan the symlink target directory since WalkDir doesn't follow symlinks
				if err := s.scanSymlinkDirectory(path); err != nil {
					logger.Error("Failed to scan symlink directory:", path, err)
				}
				return nil
			}
		}

		// Only process regular files (not directories, not symlinks to directories)
		if !d.IsDir() {
			if err := s.scanSingleFile(path); err != nil {
				logger.Error("Failed to scan config:", path, err)
			}
		}

		return nil
	})
}

// scanDirectoryRecursive implements custom recursive directory traversal
// to avoid filepath.WalkDir blocking issues on restart
func (s *Scanner) scanDirectoryRecursive(ctx context.Context, root string, fileCount, dirCount *int) error {

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

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

			// Recursively scan subdirectory
			if err := s.scanDirectoryRecursive(ctx, fullPath, fileCount, dirCount); err != nil {
				logger.Errorf("Failed to scan subdirectory %s: %v", fullPath, err)
				return err
			}
		} else {
			(*fileCount)++

			// Handle symlinks
			if entryType&os.ModeSymlink != 0 {
				targetInfo, err := os.Stat(fullPath)
				if err == nil {
					if targetInfo.IsDir() {
						// Recursively scan symlink directory
						if err := s.scanDirectoryRecursive(ctx, fullPath, fileCount, dirCount); err != nil {
							logger.Errorf("Failed to scan symlink directory %s: %v", fullPath, err)
							// Continue with other entries instead of failing completely
						}
						continue
					}
				} else {
					logger.Warnf("os.Stat failed for symlink %s: %v", fullPath, err)
				}
			}

			// Process regular files
			if err := s.scanSingleFile(fullPath); err != nil {
				logger.Errorf("Failed to scan file %s: %v", fullPath, err)
				// Continue with other files instead of failing completely
			}
		}
	}

	return nil
}

// scanSymlinkDirectory recursively scans a symlink directory and its contents
func (s *Scanner) scanSymlinkDirectory(symlinkPath string) error {
	logger.Debugf("scanSymlinkDirectory START: %s", symlinkPath)
	// Resolve the symlink to get the actual target path
	targetPath, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		logger.Errorf("Failed to resolve symlink %s: %v", symlinkPath, err)
		return fmt.Errorf("failed to resolve symlink %s: %w", symlinkPath, err)
	}

	logger.Debug("Scanning symlink directory contents:", symlinkPath, "->", targetPath)

	// Use WalkDir on the resolved target path
	walkErr := filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
		logger.Debugf("scanSymlinkDirectory callback: %s (type: %s)", path, d.Type().String())
		if err != nil {
			return err
		}

		// Skip excluded directories
		if d.IsDir() && shouldSkipPath(path) {
			return filepath.SkipDir
		}

		// Only process regular files (not directories, not symlinks to directories)
		if !d.IsDir() {
			// Handle symlinks to directories (skip them)
			if d.Type()&os.ModeSymlink != 0 {
				if targetInfo, err := os.Stat(path); err == nil && targetInfo.IsDir() {
					logger.Debug("Skipping symlink to directory in symlink scan:", path)
					return nil
				}
			}

			if err := s.scanSingleFile(path); err != nil {
				logger.Error("Failed to scan config in symlink directory:", path, err)
			}
		}
		logger.Debugf("scanSymlinkDirectory callback exit: %s", path)
		return nil
	})
	logger.Debugf("scanSymlinkDirectory END: %s -> %s (error: %v)", symlinkPath, targetPath, walkErr)
	return walkErr
}

// Shutdown cleans up scanner resources
func (s *Scanner) Shutdown() {
	logger.Info("Starting scanner shutdown...")

	// Cancel context to signal all goroutines to stop
	if s.cancel != nil {
		s.cancel()
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
	case <-time.After(10 * time.Second):
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
		case <-time.After(3 * time.Second):
			logger.Warn("Timeout waiting for scanner goroutines - proceeding with force cleanup")
		}

		scanner = nil
	}
}

// WaitForInitialScanComplete waits for the initial config scan and all callbacks to complete
// This is useful for services that depend on site indexing to be ready
func WaitForInitialScanComplete() {
	if initialScanComplete == nil {
		logger.Debug("Initial scan completion channel not initialized, returning immediately")
		return
	}

	logger.Debug("Waiting for initial config scan to complete...")

	// Add timeout to prevent infinite waiting
	select {
	case <-initialScanComplete:
		logger.Debug("Initial config scan completion confirmed")
	case <-time.After(30 * time.Second):
		logger.Warn("Timeout waiting for initial config scan completion - proceeding anyway")
	}
}
