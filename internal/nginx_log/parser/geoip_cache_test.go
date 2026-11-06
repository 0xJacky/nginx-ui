package parser

import (
	"errors"
	"fmt"
	"testing"
)

// countingGeoIPService counts underlying lookups for cache verification
type countingGeoIPService struct {
	calls int
	fail  bool
}

func (s *countingGeoIPService) Search(ip string) (*GeoLocation, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("lookup failed")
	}
	if ip == "0.0.0.0" {
		return nil, nil // not found, no error
	}
	return &GeoLocation{RegionCode: "US", Province: "California", City: "San Francisco"}, nil
}

func TestCachedGeoIPService_CacheHit(t *testing.T) {
	underlying := &countingGeoIPService{}
	cached := NewCachedGeoIPService(underlying, 100)

	first, err := cached.Search("1.2.3.4")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if first == nil || first.RegionCode != "US" {
		t.Fatalf("Search() = %v, want US location", first)
	}

	second, err := cached.Search("1.2.3.4")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if second != first {
		t.Error("expected cached pointer to be returned on second lookup")
	}
	if underlying.calls != 1 {
		t.Errorf("underlying lookups = %d, want 1", underlying.calls)
	}
}

func TestCachedGeoIPService_CachesNotFound(t *testing.T) {
	underlying := &countingGeoIPService{}
	cached := NewCachedGeoIPService(underlying, 100)

	for range 3 {
		location, err := cached.Search("0.0.0.0")
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if location != nil {
			t.Fatalf("Search() = %v, want nil for unknown IP", location)
		}
	}

	if underlying.calls != 1 {
		t.Errorf("underlying lookups = %d, want 1 (not-found should be cached)", underlying.calls)
	}
}

func TestCachedGeoIPService_ErrorsNotCached(t *testing.T) {
	underlying := &countingGeoIPService{fail: true}
	cached := NewCachedGeoIPService(underlying, 100)

	for range 2 {
		if _, err := cached.Search("1.2.3.4"); err == nil {
			t.Fatal("Search() expected error")
		}
	}

	if underlying.calls != 2 {
		t.Errorf("underlying lookups = %d, want 2 (errors must not be cached)", underlying.calls)
	}
}

func TestCachedGeoIPService_EvictionReset(t *testing.T) {
	underlying := &countingGeoIPService{}
	cached := NewCachedGeoIPService(underlying, 4)

	for i := range 10 {
		if _, err := cached.Search(fmt.Sprintf("10.0.0.%d", i)); err != nil {
			t.Fatalf("Search() error = %v", err)
		}
	}

	// The cache must stay bounded regardless of unique IP count
	cached.mu.RLock()
	size := len(cached.cache)
	cached.mu.RUnlock()
	if size > 4 {
		t.Errorf("cache size = %d, want <= 4", size)
	}
}
