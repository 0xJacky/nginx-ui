package parser

import "sync"

// CachedGeoIPService wraps a GeoIPService with a bounded lookup cache.
// Access logs contain highly repetitive IPs, so caching avoids a database
// lookup per line in the indexing hot path. Successful lookups (including
// "not found" results) are cached; errors are not.
type CachedGeoIPService struct {
	service GeoIPService
	cache   map[string]*GeoLocation
	maxSize int
	mu      sync.RWMutex
}

// NewCachedGeoIPService creates a cached GeoIP service
func NewCachedGeoIPService(service GeoIPService, maxSize int) *CachedGeoIPService {
	if maxSize <= 0 {
		maxSize = 10000
	}

	return &CachedGeoIPService{
		service: service,
		cache:   make(map[string]*GeoLocation, maxSize),
		maxSize: maxSize,
	}
}

// Search performs a cached geo IP lookup
func (c *CachedGeoIPService) Search(ip string) (*GeoLocation, error) {
	c.mu.RLock()
	if location, exists := c.cache[ip]; exists {
		c.mu.RUnlock()
		return location, nil
	}
	c.mu.RUnlock()

	location, err := c.service.Search(ip)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	// Simple eviction: reset the map when full, same policy as the
	// cached user-agent parser. Steady-state hit rate recovers quickly
	// because hot IPs re-populate within a few lines.
	if len(c.cache) >= c.maxSize {
		c.cache = make(map[string]*GeoLocation, c.maxSize)
	}
	c.cache[ip] = location
	c.mu.Unlock()

	return location, nil
}
