package sitecheck

import (
	"context"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/kernel"
	"github.com/uozi-tech/cosy/logger"
)

// Service manages site checking operations
type Service struct {
	checker         *SiteChecker
	ctx             context.Context
	cancel          context.CancelFunc
	settingsChanged chan struct{}
	mu              sync.RWMutex
	running         bool
}

var (
	globalService        *Service
	globalServiceMu      sync.RWMutex
	globalUpdateCallback func([]*SiteInfo)
)

// Init initializes the site checking service
func Init(ctx context.Context) {
	service := NewService(ctx, DefaultCheckOptions())

	globalServiceMu.Lock()
	globalService = service
	if globalUpdateCallback != nil {
		service.SetUpdateCallback(globalUpdateCallback)
	}
	globalServiceMu.Unlock()

	// Register post-scan callback to refresh sites when configs change
	cache.RegisterPostScanCallback(func() {
		service := GetService()
		if service != nil && service.IsRunning() {
			service.RefreshSites()
		}
	})

	service.Start()
}

// GetService returns the singleton service instance
func GetService() *Service {
	globalServiceMu.RLock()
	defer globalServiceMu.RUnlock()

	return globalService
}

// SetUpdateCallback registers the callback used for site status updates. The
// callback is retained when route setup runs before the service is initialized.
func SetUpdateCallback(callback func([]*SiteInfo)) {
	globalServiceMu.Lock()
	defer globalServiceMu.Unlock()

	globalUpdateCallback = callback
	if globalService != nil {
		globalService.SetUpdateCallback(callback)
	}
}

// waitForSiteCollection waits for the cache scanner to collect sites with progressive backoff
func (s *Service) waitForSiteCollection(ctx context.Context) {
	startTime := time.Now()
	logger.Debug("Waiting for site collection to complete...")

	// First, wait for the initial cache scan to complete
	// This is much more efficient than polling
	logger.Debug("Waiting for initial cache scan to complete before site collection...")
	cache.WaitForInitialScanComplete()
	logger.Debugf("Initial cache scan completed after %v, now collecting sites", time.Since(startTime))

	// Now collect sites - the cache scanning should have populated IndexedSites
	s.checker.CollectSites()
	siteCount := s.checker.GetSiteCount()
	logger.Debugf("Site collection completed: found %d sites after %v", siteCount, time.Since(startTime))

	// If no sites found after cache scan, do a brief fallback check
	if siteCount == 0 {
		logger.Debug("No sites found after cache scan completion, doing fallback check...")
		maxWaitTime := 10 * time.Second  // Reduced from 30s since cache scan already completed
		checkInterval := 2 * time.Second // Reduced interval

		for {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				logger.Debug("Site collection fallback wait cancelled")
				return
			default:
			}

			// Re-check for sites
			s.checker.CollectSites()
			siteCount = s.checker.GetSiteCount()

			logger.Debugf("Fallback site collection check: found %d sites (total waited %v)",
				siteCount, time.Since(startTime))

			if siteCount > 0 {
				logger.Debugf("Site collection completed via fallback: found %d sites", siteCount)
				return
			}

			// Check if we've exceeded max fallback wait time
			if time.Since(startTime) >= maxWaitTime {
				logger.Debugf("Site collection fallback timeout after %v - proceeding with empty site list",
					time.Since(startTime))
				return
			}

			// Wait before next check
			select {
			case <-ctx.Done():
				return
			case <-time.After(checkInterval):
				// Continue to next iteration
			}
		}
	}
}

// NewService creates a new site checking service
func NewService(ctx context.Context, options CheckOptions) *Service {
	return NewServiceWithContext(ctx, options)
}

// NewServiceWithContext creates a new site checking service with a parent context
func NewServiceWithContext(parentCtx context.Context, options CheckOptions) *Service {
	ctx, cancel := context.WithCancel(parentCtx)

	return &Service{
		checker:         NewSiteChecker(options),
		ctx:             ctx,
		cancel:          cancel,
		settingsChanged: make(chan struct{}, 1),
	}
}

// SetUpdateCallback sets the callback function for site updates
func (s *Service) SetUpdateCallback(callback func([]*SiteInfo)) {
	s.checker.SetUpdateCallback(callback)
}

// Start begins site discovery and periodic checking. Discovery stays active
// while probes are globally disabled so the dashboard can still show the
// configured sites without generating network traffic.
func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true

	// Initial collection and check with delay to allow cache scanner to complete
	go kernel.Run(s.ctx, "sitecheck initial collection goroutine", func(ctx context.Context) {
		sl := logger.NewSessionLogger(ctx)
		sl.Debug("Started sitecheck initial collection goroutine")
		// Give cache scanner more time to start up before checking
		time.Sleep(5 * time.Second)

		// Wait for cache scanner to collect sites with progressive backoff
		s.waitForSiteCollection(s.ctx)
		s.checker.ForceCheckAllSites(s.ctx)
		sl.Debug("Sitecheck initial collection goroutine completed")
	})

	// Start periodic checking using the configured interval.
	go kernel.Run(s.ctx, "sitecheck periodic check goroutine", func(ctx context.Context) {
		sl := logger.NewSessionLogger(ctx)
		sl.Debug("Started sitecheck periodicCheck goroutine")
		s.periodicCheck()
		sl.Debug("Sitecheck periodicCheck goroutine completed")
	})
}

// Stop stops the site checking service
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	sl := logger.NewSessionLogger(s.ctx)

	s.running = false
	sl.Debug("Stopping site checking service")

	s.cancel()
}

// Restart restarts the site checking service
func (s *Service) Restart() {
	s.Stop()
	time.Sleep(100 * time.Millisecond) // Brief pause
	s.Start()
}

// periodicCheck runs periodic site checks
func (s *Service) periodicCheck() {
	sl := logger.NewSessionLogger(s.ctx)
	for {
		timer := time.NewTimer(settings.SiteCheckSettings.GetInterval())
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-s.settingsChanged:
			if !timer.Stop() {
				<-timer.C
			}
			continue
		case <-timer.C:
			sl.Debug("Starting periodic site check")
			s.checker.CollectSites() // Re-collect in case sites changed
			s.checker.CheckAllSites(s.ctx)
		}
	}
}

// SettingsChanged applies site-check settings without restarting the process.
// The current timer is interrupted so a new interval takes effect immediately,
// and a refresh broadcasts the new effective enabled state to clients.
func (s *Service) SettingsChanged() {
	select {
	case s.settingsChanged <- struct{}{}:
	default:
	}
	s.RefreshSites()
}

// RefreshSites manually triggers a site collection and check
func (s *Service) RefreshSites() {
	go func() {
		sl := logger.NewSessionLogger(s.ctx)
		sl.Debug("Started sitecheck refresh goroutine")
		s.checker.CollectSites()
		s.checker.ForceCheckAllSites(s.ctx)
		sl.Debug("Sitecheck refresh goroutine completed")
	}()
}

// GetSites returns all checked sites with custom ordering applied
func (s *Service) GetSites() []*SiteInfo {
	sites := s.checker.GetSitesList()

	// Apply custom ordering from database
	return s.applySiteOrdering(sites)
}

// GetSiteByURL returns a specific site by URL
func (s *Service) GetSiteByURL(url string) *SiteInfo {
	sites := s.checker.GetSites()
	if site, exists := sites[url]; exists {
		return site
	}
	return nil
}

// IsRunning returns whether the service is currently running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// applySiteOrdering applies custom ordering from database to sites
func (s *Service) applySiteOrdering(sites []*SiteInfo) []*SiteInfo {
	return applyCustomOrdering(sites)
}
