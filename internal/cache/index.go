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

// Scanner watches and scans nginx config files
type Scanner struct {
	ctx        context.Context
	watcher    *fsnotify.Watcher
	scanTicker *time.Ticker
	scanning   bool
	scanMutex  sync.RWMutex
}

var (
	scanner            *Scanner
	scannerInitMutex   sync.Mutex
	scanCallbacks      = make([]ScanCallback, 0)
	scanCallbacksMutex sync.RWMutex
)

// InitScanner initializes the config scanner
func InitScanner(ctx context.Context) {
	if nginx.GetConfPath() == "" {
		logger.Error("Nginx config path is not set")
		return
	}

	scanner := GetScanner()
	if err := scanner.Initialize(ctx); err != nil {
		logger.Error("Failed to initialize config scanner:", err)
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

// RegisterCallback adds a callback to be executed during scans
func RegisterCallback(callback ScanCallback) {
	scanCallbacksMutex.Lock()
	defer scanCallbacksMutex.Unlock()
	scanCallbacks = append(scanCallbacks, callback)
}

// Initialize sets up the scanner and starts watching
func (s *Scanner) Initialize(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	s.watcher = watcher
	s.ctx = ctx

	// Initial scan
	if err := s.ScanAllConfigs(); err != nil {
		return err
	}

	// Watch all directories recursively
	if err := s.watchAllDirectories(); err != nil {
		return err
	}

	// Start background processes
	go s.watchForChanges()
	go s.periodicScan()
	go s.handleShutdown()

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
			logger.Debug("Watching directory:", actualPath)
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
	logger.Info("Shutting down Index Scanner")
	s.Shutdown()
}

// watchForChanges handles file system events
func (s *Scanner) watchForChanges() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.handleFileEvent(event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
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
		logger.Debugf("Processing symlink to file: %s", filePath)
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
			Type: event.EventTypeIndexScanning,
			Data: scanning,
		})
	}
}

// executeCallbacks runs all registered callbacks
func (s *Scanner) executeCallbacks(filePath string, content []byte) {
	scanCallbacksMutex.RLock()
	defer scanCallbacksMutex.RUnlock()

	for _, callback := range scanCallbacks {
		if err := callback(filePath, content); err != nil {
			logger.Error("Callback error for", filePath, ":", err)
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

// scanSymlinkDirectory recursively scans a symlink directory and its contents
func (s *Scanner) scanSymlinkDirectory(symlinkPath string) error {
	// Resolve the symlink to get the actual target path
	targetPath, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink %s: %w", symlinkPath, err)
	}

	logger.Debug("Scanning symlink directory contents:", symlinkPath, "->", targetPath)

	// Use WalkDir on the resolved target path
	return filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
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
		return nil
	})
}

// Shutdown cleans up scanner resources
func (s *Scanner) Shutdown() {
	if s.watcher != nil {
		s.watcher.Close()
	}
	if s.scanTicker != nil {
		s.scanTicker.Stop()
	}
}

// IsScanningInProgress returns whether a scan is currently running
func IsScanningInProgress() bool {
	s := GetScanner()
	s.scanMutex.RLock()
	defer s.scanMutex.RUnlock()
	return s.scanning
}
