package cache

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
)

// ScanCallback is a function that gets called during config scanning
// It receives the config file path and contents
type ScanCallback func(configPath string, content []byte) error

// Scanner is responsible for scanning and watching nginx config files
type Scanner struct {
	ctx           context.Context        // Context for the scanner
	watcher       *fsnotify.Watcher      // File system watcher
	scanTicker    *time.Ticker           // Ticker for periodic scanning
	initialized   bool                   // Whether the scanner has been initialized
	scanning      bool                   // Whether a scan is currently in progress
	scanMutex     sync.RWMutex           // Mutex for protecting the scanning state
	statusChan    chan bool              // Channel to broadcast scanning status changes
	subscribers   map[chan bool]struct{} // Set of subscribers
	subscriberMux sync.RWMutex           // Mutex for protecting the subscribers map
}

// Global variables
var (
	// scanner is the singleton instance of Scanner
	scanner              *Scanner
	configScannerInitMux sync.Mutex

	// This regex matches: include directives in nginx config files
	includeRegex = regexp.MustCompile(`include\s+([^;]+);`)

	// Global callbacks that will be executed during config file scanning
	scanCallbacks      = make([]ScanCallback, 0)
	scanCallbacksMutex sync.RWMutex
)

// InitScanner initializes the config scanner
func InitScanner(ctx context.Context) {
	if nginx.GetConfPath() == "" {
		logger.Error("Nginx config path is not set")
		return
	}

	s := GetScanner()
	err := s.Initialize(ctx)
	if err != nil {
		logger.Error("Failed to initialize config scanner:", err)
	}
}

// GetScanner returns the singleton instance of Scanner
func GetScanner() *Scanner {
	configScannerInitMux.Lock()
	defer configScannerInitMux.Unlock()

	if scanner == nil {
		scanner = &Scanner{
			statusChan:  make(chan bool, 10), // Buffer to prevent blocking
			subscribers: make(map[chan bool]struct{}),
		}

		// Start broadcaster goroutine
		go scanner.broadcastStatus()
	}
	return scanner
}

// RegisterCallback adds a callback function to be executed during scans
// This function can be called before Scanner is initialized
func RegisterCallback(callback ScanCallback) {
	scanCallbacksMutex.Lock()
	defer scanCallbacksMutex.Unlock()
	scanCallbacks = append(scanCallbacks, callback)
}

// broadcastStatus listens for status changes and broadcasts to all subscribers
func (s *Scanner) broadcastStatus() {
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

// SubscribeScanningStatus allows a client to subscribe to scanning status changes
func SubscribeScanningStatus() chan bool {
	s := GetScanner()
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

// UnsubscribeScanningStatus removes a subscriber from receiving status updates
func UnsubscribeScanningStatus(ch chan bool) {
	s := GetScanner()

	s.subscriberMux.Lock()
	delete(s.subscribers, ch)
	s.subscriberMux.Unlock()

	// Close the channel so the client knows it's unsubscribed
	close(ch)
}

// Initialize sets up the scanner and starts watching for file changes
func (s *Scanner) Initialize(ctx context.Context) error {
	if s.initialized {
		return nil
	}

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	s.watcher = watcher
	s.ctx = ctx

	// Scan for the first time
	err = s.ScanAllConfigs()
	if err != nil {
		return err
	}

	// Setup watcher for config directory
	configDir := filepath.Dir(nginx.GetConfPath())
	availableDir := nginx.GetConfPath("sites-available")
	enabledDir := nginx.GetConfPath("sites-enabled")
	streamAvailableDir := nginx.GetConfPath("stream-available")
	streamEnabledDir := nginx.GetConfPath("stream-enabled")

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
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.scanTicker.C:
				err := s.ScanAllConfigs()
				if err != nil {
					logger.Error("Periodic config scan failed:", err)
				}
			}
		}
	}()

	// Start a goroutine to listen for context cancellation
	go func() {
		<-s.ctx.Done()
		logger.Debug("Context cancelled, shutting down scanner")
		s.Shutdown()
	}()

	s.initialized = true
	return nil
}

// watchForChanges handles the fsnotify events and triggers rescans when necessary
func (s *Scanner) watchForChanges() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Skip irrelevant events
			if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) &&
				!event.Has(fsnotify.Rename) && !event.Has(fsnotify.Remove) {
				continue
			}

			// Add newly created directories to the watch list
			if event.Has(fsnotify.Create) {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					_ = s.watcher.Add(event.Name)
				}
			}

			// For remove events, perform a full scan
			if event.Has(fsnotify.Remove) {
				logger.Debug("Config item removed:", event.Name)
				if err := s.ScanAllConfigs(); err != nil {
					logger.Error("Failed to rescan configs after removal:", err)
				}
				continue
			}

			// Handle non-remove events
			fi, err := os.Stat(event.Name)
			if err != nil {
				logger.Error("Failed to stat changed path:", err)
				continue
			}

			if fi.IsDir() {
				// Directory change, perform full scan
				logger.Debug("Config directory changed:", event.Name)
				if err := s.ScanAllConfigs(); err != nil {
					logger.Error("Failed to rescan configs after directory change:", err)
				}
			} else {
				// File change, scan only the single file
				logger.Debug("Config file changed:", event.Name)
				// Give the system a moment to finish writing the file
				time.Sleep(100 * time.Millisecond)
				if err := s.scanSingleFile(event.Name); err != nil {
					logger.Error("Failed to scan changed file:", err)
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

// scanSingleFile scans a single file and executes all registered callbacks
func (s *Scanner) scanSingleFile(filePath string) error {
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

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read the entire file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Execute all registered callbacks
	scanCallbacksMutex.RLock()
	for _, callback := range scanCallbacks {
		err := callback(filePath, content)
		if err != nil {
			logger.Error("Callback error for file", filePath, ":", err)
		}
	}
	scanCallbacksMutex.RUnlock()

	// Look for include directives to process included files
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
						err = s.scanSingleFile(matchedFile)
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
					err = s.scanSingleFile(includePath)
					if err != nil {
						logger.Error("Failed to scan included file:", includePath, err)
					}
				}
			}
		}
	}

	return nil
}

// ScanAllConfigs scans all nginx config files and executes all registered callbacks
func (s *Scanner) ScanAllConfigs() error {
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

	// Get the main config file
	mainConfigPath := nginx.GetConfPath("", "nginx.conf")
	err := s.scanSingleFile(mainConfigPath)
	if err != nil {
		logger.Error("Failed to scan main config:", err)
	}

	// Scan sites-available directory
	sitesAvailablePath := nginx.GetConfPath("sites-available", "")
	sitesAvailableFiles, err := os.ReadDir(sitesAvailablePath)
	if err == nil {
		for _, file := range sitesAvailableFiles {
			if !file.IsDir() {
				configPath := filepath.Join(sitesAvailablePath, file.Name())
				err := s.scanSingleFile(configPath)
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
				err := s.scanSingleFile(configPath)
				if err != nil {
					logger.Error("Failed to scan stream config:", configPath, err)
				}
			}
		}
	}

	return nil
}

// Shutdown cleans up resources used by the scanner
func (s *Scanner) Shutdown() {
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

// IsScanningInProgress returns whether a scan is currently in progress
func IsScanningInProgress() bool {
	s := GetScanner()
	s.scanMutex.RLock()
	defer s.scanMutex.RUnlock()
	return s.scanning
}

// WithContext sets a context for the scanner that will be used to control its lifecycle
func (s *Scanner) WithContext(ctx context.Context) *Scanner {
	// Create a context with cancel if not already done in Initialize
	if s.ctx == nil {
		s.ctx = ctx
	}
	return s
}
