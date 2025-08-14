package sitecheck

import (
	"context"
	"sync"
	"time"

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
	serviceOnce   sync.Once
)

// GetService returns the singleton service instance
func GetService() *Service {
	serviceOnce.Do(func() {
		globalService = NewService(DefaultCheckOptions())
	})
	return globalService
}

// NewService creates a new site checking service
func NewService(options CheckOptions) *Service {
	return NewServiceWithContext(context.Background(), options)
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
	logger.Info("Starting site checking service")

	// Initial collection and check with delay to allow cache scanner to complete
	go func() {
		// Wait a bit for cache scanner to collect sites
		time.Sleep(2 * time.Second)
		s.checker.CollectSites()
		s.checker.CheckAllSites(s.ctx)
	}()

	// Start periodic checking (every 5 minutes)
	s.ticker = time.NewTicker(5 * time.Minute)
	go s.periodicCheck()
}

// Stop stops the site checking service
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	logger.Info("Stopping site checking service")

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
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			logger.Debug("Starting periodic site check")
			s.checker.CollectSites() // Re-collect in case sites changed
			s.checker.CheckAllSites(s.ctx)
		}
	}
}

// RefreshSites manually triggers a site collection and check
func (s *Service) RefreshSites() {
	go func() {
		logger.Info("Manually refreshing sites")
		s.checker.CollectSites()
		s.checker.CheckAllSites(s.ctx)
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
