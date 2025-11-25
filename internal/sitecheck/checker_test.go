package sitecheck

import (
	"context"
	"net/http"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCheckSiteSkipsNetworkWhenDisabled(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)

	options := DefaultCheckOptions()
	checker := NewSiteChecker(options)

	// Any HTTP request made by the checker should fail this test.
	checker.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request to %s while health check is disabled", req.URL.String())
			return nil, nil
		}),
	}

	const siteURL = "https://example.com"
	config := &model.SiteConfig{
		Model:              model.Model{ID: 1},
		Host:               "example.com:443",
		Scheme:             "https",
		DisplayURL:         siteURL,
		HealthCheckEnabled: false,
		HealthCheckConfig: &model.HealthCheckConfig{
			Protocol: "https",
		},
	}

	setCachedSiteConfig(config.Host, config)

	if _, err := checker.CheckSite(context.Background(), siteURL); err != nil {
		t.Fatalf("CheckSite returned error: %v", err)
	}
}
