package cache

import (
	"context"
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
	sslDir := nginx.GetConfPath("ssl")

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip ssl directory
			if path == sslDir {
				return filepath.SkipDir
			}

			if err := s.watcher.Add(path); err != nil {
				logger.Error("Failed to watch directory:", path, err)
				return err
			}
			logger.Debug("Watching directory:", path)
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
	logger.Debug("Shutting down scanner")
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

	// Skip ssl directory
	sslDir := nginx.GetConfPath("ssl")
	if strings.HasPrefix(event.Name, sslDir) {
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

	fi, err := os.Stat(event.Name)
	if err != nil {
		return
	}

	if fi.IsDir() {
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
	sslDir := nginx.GetConfPath("ssl")

	// Scan all files in the config directory and subdirectories
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip ssl directory
		if d.IsDir() && path == sslDir {
			return filepath.SkipDir
		}

		// Only process regular files
		if !d.IsDir() {
			if err := s.scanSingleFile(path); err != nil {
				logger.Error("Failed to scan config:", path, err)
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
