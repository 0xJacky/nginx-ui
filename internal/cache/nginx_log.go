package cache

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
)

// NginxLogCache represents a cached log entry from nginx configuration
type NginxLogCache struct {
	Path string `json:"path"` // Path to the log file
	Type string `json:"type"` // Type of log: "access" or "error"
	Name string `json:"name"` // Name of the log file
}

// NginxLogScanner is responsible for scanning and watching nginx config files for log directives
type NginxLogScanner struct {
	logCache      map[string]*NginxLogCache // Map of log path to cache entry
	cacheMutex    sync.RWMutex              // Mutex for protecting the cache
	watcher       *fsnotify.Watcher         // File system watcher
	scanTicker    *time.Ticker              // Ticker for periodic scanning
	initialized   bool                      // Whether the scanner has been initialized
	scanning      bool                      // Whether a scan is currently in progress
	scanMutex     sync.RWMutex              // Mutex for protecting the scanning state
	statusChan    chan bool                 // Channel to broadcast scanning status changes
	subscribers   map[chan bool]struct{}    // Set of subscribers
	subscriberMux sync.RWMutex              // Mutex for protecting the subscribers map
}

// Add regex constants at package level
var (
	// logScanner is the singleton instance of NginxLogScanner
	logScanner     *NginxLogScanner
	scannerInitMux sync.Mutex
)

// Compile the regular expressions for matching log directives
var (
	// This regex matches: access_log or error_log, followed by a path, and optional parameters ending with semicolon
	logDirectiveRegex = regexp.MustCompile(`(?m)(access_log|error_log)\s+([^\s;]+)(?:\s+[^;]+)?;`)
)

// InitNginxLogScanner initializes the nginx log scanner
func InitNginxLogScanner() {
	scanner := GetNginxLogScanner()
	err := scanner.Initialize()
	if err != nil {
		logger.Error("Failed to initialize nginx log scanner:", err)
	}
}

// GetNginxLogScanner returns the singleton instance of NginxLogScanner
func GetNginxLogScanner() *NginxLogScanner {
	scannerInitMux.Lock()
	defer scannerInitMux.Unlock()

	if logScanner == nil {
		logScanner = &NginxLogScanner{
			logCache:    make(map[string]*NginxLogCache),
			statusChan:  make(chan bool, 10), // Buffer to prevent blocking
			subscribers: make(map[chan bool]struct{}),
		}

		// Start broadcaster goroutine
		go logScanner.broadcastStatus()
	}
	return logScanner
}

// broadcastStatus listens for status changes and broadcasts to all subscribers
func (s *NginxLogScanner) broadcastStatus() {
	for status := range s.statusChan {
		s.subscriberMux.RLock()
		for ch := range s.subscribers {
			// Non-blocking send to prevent slow subscribers from blocking others
			select {
			case ch <- status:
			default:
				// Skip if channel buffer is full
			}
		}
		s.subscriberMux.RUnlock()
	}
}

// SubscribeStatusChanges allows a client to subscribe to scanning status changes
func SubscribeStatusChanges() chan bool {
	s := GetNginxLogScanner()
	ch := make(chan bool, 5) // Buffer to prevent blocking

	// Add to subscribers
	s.subscriberMux.Lock()
	s.subscribers[ch] = struct{}{}
	s.subscriberMux.Unlock()

	// Send current status immediately
	s.scanMutex.RLock()
	currentStatus := s.scanning
	s.scanMutex.RUnlock()

	// Non-blocking send
	select {
	case ch <- currentStatus:
	default:
	}

	return ch
}

// UnsubscribeStatusChanges removes a subscriber from receiving status updates
func UnsubscribeStatusChanges(ch chan bool) {
	s := GetNginxLogScanner()

	s.subscriberMux.Lock()
	delete(s.subscribers, ch)
	s.subscriberMux.Unlock()

	// Close the channel so the client knows it's unsubscribed
	close(ch)
}

// Initialize sets up the log scanner and starts watching for file changes
func (s *NginxLogScanner) Initialize() error {
	if s.initialized {
		return nil
	}

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	s.watcher = watcher

	// Scan for the first time
	err = s.ScanAllConfigs()
	if err != nil {
		return err
	}

	// Setup watcher for config directory
	configDir := filepath.Dir(nginx.GetConfPath("", ""))
	availableDir := nginx.GetConfPath("sites-available", "")
	enabledDir := nginx.GetConfPath("sites-enabled", "")
	streamAvailableDir := nginx.GetConfPath("stream-available", "")
	streamEnabledDir := nginx.GetConfPath("stream-enabled", "")

	// Watch the main directories
	err = s.watcher.Add(configDir)
	if err != nil {
		logger.Error("Failed to watch config directory:", err)
	}

	// Watch sites-available and sites-enabled if they exist
	if _, err := os.Stat(availableDir); err == nil {
		err = s.watcher.Add(availableDir)
		if err != nil {
			logger.Error("Failed to watch sites-available directory:", err)
		}
	}

	if _, err := os.Stat(enabledDir); err == nil {
		err = s.watcher.Add(enabledDir)
		if err != nil {
			logger.Error("Failed to watch sites-enabled directory:", err)
		}
	}

	// Watch stream-available and stream-enabled if they exist
	if _, err := os.Stat(streamAvailableDir); err == nil {
		err = s.watcher.Add(streamAvailableDir)
		if err != nil {
			logger.Error("Failed to watch stream-available directory:", err)
		}
	}

	if _, err := os.Stat(streamEnabledDir); err == nil {
		err = s.watcher.Add(streamEnabledDir)
		if err != nil {
			logger.Error("Failed to watch stream-enabled directory:", err)
		}
	}

	// Start the watcher goroutine
	go s.watchForChanges()

	// Setup a ticker for periodic scanning (every 5 minutes)
	s.scanTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for range s.scanTicker.C {
			err := s.ScanAllConfigs()
			if err != nil {
				logger.Error("Periodic config scan failed:", err)
			}
		}
	}()

	s.initialized = true
	return nil
}

// watchForChanges handles the fsnotify events and triggers rescans when necessary
func (s *NginxLogScanner) watchForChanges() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Check if this is a relevant event (create, write, rename, remove)
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) ||
				event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				// If it's a directory, add it to the watch list
				if event.Has(fsnotify.Create) {
					fi, err := os.Stat(event.Name)
					if err == nil && fi.IsDir() {
						_ = s.watcher.Add(event.Name)
					}
				}

				// Process file changes - no .conf restriction anymore
				if !event.Has(fsnotify.Remove) {
					logger.Debug("Config file changed:", event.Name)
					// Give the system a moment to finish writing the file
					time.Sleep(100 * time.Millisecond)
					// Only scan the changed file instead of all configs
					err := s.scanSingleFile(event.Name)
					if err != nil {
						logger.Error("Failed to scan changed file:", err)
					}
				} else {
					// For removed files, we need to clean up any log entries that came from this file
					// This would require tracking which logs came from which config files
					// For now, we'll do a full rescan which is simpler but less efficient
					err := s.ScanAllConfigs()
					if err != nil {
						logger.Error("Failed to rescan configs after file removal:", err)
					}
				}
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("Watcher error:", err)
		}
	}
}

// scanSingleFile scans a single file and updates the log cache accordingly
func (s *NginxLogScanner) scanSingleFile(filePath string) error {
	// Set scanning state to true
	s.scanMutex.Lock()
	wasScanning := s.scanning
	s.scanning = true
	if !wasScanning {
		// Only broadcast if status changed from not scanning to scanning
		s.statusChan <- true
	}
	s.scanMutex.Unlock()

	// Ensure we reset scanning state when done
	defer func() {
		s.scanMutex.Lock()
		s.scanning = false
		// Broadcast the completion
		s.statusChan <- false
		s.scanMutex.Unlock()
	}()

	// Create a temporary cache for new entries from this file
	newEntries := make(map[string]*NginxLogCache)

	// Scan the file
	err := s.scanConfigFile(filePath, newEntries)
	if err != nil {
		return err
	}

	// Update the main cache with new entries
	s.cacheMutex.Lock()
	for path, entry := range newEntries {
		s.logCache[path] = entry
	}
	s.cacheMutex.Unlock()

	return nil
}

// ScanAllConfigs scans all nginx config files for log directives
func (s *NginxLogScanner) ScanAllConfigs() error {
	// Set scanning state to true
	s.scanMutex.Lock()
	wasScanning := s.scanning
	s.scanning = true
	if !wasScanning {
		// Only broadcast if status changed from not scanning to scanning
		s.statusChan <- true
	}
	s.scanMutex.Unlock()

	// Ensure we reset scanning state when done
	defer func() {
		s.scanMutex.Lock()
		s.scanning = false
		// Broadcast the completion
		s.statusChan <- false
		s.scanMutex.Unlock()
	}()

	// Initialize a new cache to replace the old one
	newCache := make(map[string]*NginxLogCache)

	// Get the main config file
	mainConfigPath := nginx.GetConfPath("", "nginx.conf")
	err := s.scanConfigFile(mainConfigPath, newCache)
	if err != nil {
		logger.Error("Failed to scan main config:", err)
	}

	// Scan sites-available directory - no .conf restriction anymore
	sitesAvailablePath := nginx.GetConfPath("sites-available", "")
	sitesAvailableFiles, err := os.ReadDir(sitesAvailablePath)
	if err == nil {
		for _, file := range sitesAvailableFiles {
			if !file.IsDir() {
				configPath := filepath.Join(sitesAvailablePath, file.Name())
				err := s.scanConfigFile(configPath, newCache)
				if err != nil {
					logger.Error("Failed to scan config:", configPath, err)
				}
			}
		}
	}

	// Scan stream-available directory if it exists
	streamAvailablePath := nginx.GetConfPath("stream-available", "")
	streamAvailableFiles, err := os.ReadDir(streamAvailablePath)
	if err == nil {
		for _, file := range streamAvailableFiles {
			if !file.IsDir() {
				configPath := filepath.Join(streamAvailablePath, file.Name())
				err := s.scanConfigFile(configPath, newCache)
				if err != nil {
					logger.Error("Failed to scan stream config:", configPath, err)
				}
			}
		}
	}

	// Replace the old cache with the new one
	s.cacheMutex.Lock()
	s.logCache = newCache
	s.cacheMutex.Unlock()

	return nil
}

// scanConfigFile scans a single config file for log directives using regex
func (s *NginxLogScanner) scanConfigFile(configPath string, cache map[string]*NginxLogCache) error {
	// Open the file
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read the entire file content
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Find all matches of log directives
	matches := logDirectiveRegex.FindAllSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			directiveType := string(match[1]) // "access_log" or "error_log"
			logPath := string(match[2])       // The log file path

			// Validate the log path
			if isValidLogPath(logPath) {
				logType := "access"
				if directiveType == "error_log" {
					logType = "error"
				}

				cache[logPath] = &NginxLogCache{
					Path: logPath,
					Type: logType,
					Name: filepath.Base(logPath),
				}
			}
		}
	}

	// Look for include directives to process included files
	includeRegex := regexp.MustCompile(`include\s+([^;]+);`)
	includeMatches := includeRegex.FindAllSubmatch(content, -1)

	for _, match := range includeMatches {
		if len(match) >= 2 {
			includePath := string(match[1])
			// Handle glob patterns in include directives
			if strings.Contains(includePath, "*") {
				// If it's a relative path, make it absolute based on nginx config dir
				if !filepath.IsAbs(includePath) {
					configDir := filepath.Dir(nginx.GetConfPath("", ""))
					includePath = filepath.Join(configDir, includePath)
				}

				// Expand the glob pattern
				matchedFiles, err := filepath.Glob(includePath)
				if err != nil {
					logger.Error("Error expanding glob pattern:", includePath, err)
					continue
				}

				// Process each matched file
				for _, matchedFile := range matchedFiles {
					fileInfo, err := os.Stat(matchedFile)
					if err == nil && !fileInfo.IsDir() {
						err = s.scanConfigFile(matchedFile, cache)
						if err != nil {
							logger.Error("Failed to scan included file:", matchedFile, err)
						}
					}
				}
			} else {
				// Handle single file include
				// If it's a relative path, make it absolute based on nginx config dir
				if !filepath.IsAbs(includePath) {
					configDir := filepath.Dir(nginx.GetConfPath("", ""))
					includePath = filepath.Join(configDir, includePath)
				}

				fileInfo, err := os.Stat(includePath)
				if err == nil && !fileInfo.IsDir() {
					err = s.scanConfigFile(includePath, cache)
					if err != nil {
						logger.Error("Failed to scan included file:", includePath, err)
					}
				}
			}
		}
	}

	return nil
}

// isLogPathUnderWhiteList checks if the log path is under one of the paths in LogDirWhiteList
// This is a duplicate of the function in nginx_log package to avoid import cycle
func isLogPathUnderWhiteList(path string) bool {
	// deep copy
	logDirWhiteList := append([]string{}, settings.NginxSettings.LogDirWhiteList...)

	accessLogPath := nginx.GetAccessLogPath()
	errorLogPath := nginx.GetErrorLogPath()

	if accessLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(accessLogPath))
	}
	if errorLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(errorLogPath))
	}

	for _, whitePath := range logDirWhiteList {
		if helper.IsUnderDirectory(path, whitePath) {
			return true
		}
	}
	return false
}

// isValidLogPath checks if a log path is valid:
// 1. It must be a regular file or a symlink to a regular file
// 2. It must not point to a console or special device
// 3. It must be under the whitelist directories
func isValidLogPath(logPath string) bool {
	// First check if the path is under the whitelist
	if !isLogPathUnderWhiteList(logPath) {
		logger.Warn("Log path is not under whitelist:", logPath)
		return false
	}

	// Check if the path exists
	fileInfo, err := os.Lstat(logPath)
	if err != nil {
		// If file doesn't exist, it might be created later
		// We'll assume it's valid for now
		return true
	}

	// If it's a symlink, follow it
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(logPath)
		if err != nil {
			return false
		}

		// Make absolute path if the link target is relative
		if !filepath.IsAbs(linkTarget) {
			linkTarget = filepath.Join(filepath.Dir(logPath), linkTarget)
		}

		// Check the target file
		targetInfo, err := os.Stat(linkTarget)
		if err != nil {
			return false
		}

		// Only accept regular files as targets
		return targetInfo.Mode().IsRegular()
	}

	// For non-symlinks, just check if it's a regular file
	return fileInfo.Mode().IsRegular()
}

// Shutdown cleans up resources used by the scanner
func (s *NginxLogScanner) Shutdown() {
	if s.watcher != nil {
		s.watcher.Close()
	}

	if s.scanTicker != nil {
		s.scanTicker.Stop()
	}

	// Clean up subscriber resources
	s.subscriberMux.Lock()
	// Close all subscriber channels
	for ch := range s.subscribers {
		close(ch)
	}
	// Clear the map
	s.subscribers = make(map[chan bool]struct{})
	s.subscriberMux.Unlock()

	// Close the status channel
	close(s.statusChan)
}

// GetAllLogPaths returns all cached log paths
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	s := GetNginxLogScanner()
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	result := make([]*NginxLogCache, 0, len(s.logCache))
	for _, cache := range s.logCache {
		flag := true
		if len(filters) > 0 {
			for _, filter := range filters {
				if !filter(cache) {
					flag = false
					break
				}
			}
		}
		if flag {
			result = append(result, cache)
		}
	}

	return result
}

// IsScanning returns whether a scan is currently in progress
func IsScanning() bool {
	s := GetNginxLogScanner()
	s.scanMutex.RLock()
	defer s.scanMutex.RUnlock()
	return s.scanning
}
