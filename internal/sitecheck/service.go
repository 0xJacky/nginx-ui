package sitecheck

import (
	"context"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/uozi-tech/cosy/kernel"
	"github.com/uozi-tech/cosy/logger"
)

// Service manages site checking operations
type Service struct {
	checker *SiteChecker
	ctx     context.Context
	cancel  context.CancelFunc
	ticker  *time.Ticker
	mu      sync.RWMutex
	running bool
}

var (
	globalService *Service
)

// Init initializes the site checking service
func Init(ctx context.Context) {
	globalService = NewService(ctx, DefaultCheckOptions())

	globalService.Start()
}

// GetService returns the singleton service instance
func GetService() *Service {
	return globalService
}

// waitForSiteCollection waits for the cache scanner to collect sites with progressive backoff
func (s *Service) waitForSiteCollection(ctx context.Context) {
	startTime := time.Now()
	logger.Debug("Waiting for site collection to complete...")

	// First, wait for the initial cache scan to complete
	// This is much more efficient than polling
	logger.Info("Waiting for initial cache scan to complete before site collection...")
	cache.WaitForInitialScanComplete()
	logger.Infof("Initial cache scan completed after %v, now collecting sites", time.Since(startTime))

	// Now collect sites - the cache scanning should have populated IndexedSites
	s.checker.CollectSites()
	siteCount := s.checker.GetSiteCount()
	logger.Infof("Site collection completed: found %d sites after %v", siteCount, time.Since(startTime))

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
				logger.Warnf("Site collection completed via fallback: found %d sites", siteCount)
				return
			}

			// Check if we've exceeded max fallback wait time
			if time.Since(startTime) >= maxWaitTime {
				logger.Warnf("Site collection fallback timeout after %v - proceeding with empty site list",
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
		checker: NewSiteChecker(options),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// SetUpdateCallback sets the callback function for site updates
func (s *Service) SetUpdateCallback(callback func([]*SiteInfo)) {
	s.checker.SetUpdateCallback(callback)
}

// Start begins the site checking service
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
		sl.Info("Started sitecheck initial collection goroutine")
		// Give cache scanner more time to start up before checking
		time.Sleep(5 * time.Second)

		// Wait for cache scanner to collect sites with progressive backoff
		s.waitForSiteCollection(s.ctx)
		s.checker.CheckAllSites(s.ctx)
		sl.Info("Sitecheck initial collection goroutine completed")
	})

	// Start periodic checking (every 5 minutes)
	s.ticker = time.NewTicker(5 * time.Minute)
	go kernel.Run(s.ctx, "sitecheck periodic check goroutine", func(ctx context.Context) {
		sl := logger.NewSessionLogger(ctx)
		sl.Info("Started sitecheck periodicCheck goroutine")
		s.periodicCheck()
		sl.Info("Sitecheck periodicCheck goroutine completed")
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
	sl.Info("Stopping site checking service")

	if s.ticker != nil {
		s.ticker.Stop()
	}
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
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			sl.Debug("Starting periodic site check")
			s.checker.CollectSites() // Re-collect in case sites changed
			s.checker.CheckAllSites(s.ctx)
		}
	}
}

// RefreshSites manually triggers a site collection and check
func (s *Service) RefreshSites() {
	go func() {
		sl := logger.NewSessionLogger(s.ctx)
		sl.Info("Started sitecheck manual refresh goroutine")
		logger.Info("Manually refreshing sites")
		s.checker.CollectSites()
		s.checker.CheckAllSites(s.ctx)
		sl.Info("Sitecheck manual refresh goroutine completed")
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
