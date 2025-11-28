package sitecheck

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestDownloadFaviconAcceptsValidImage(t *testing.T) {
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0x0F, 0x04, 0x00,
		0x09, 0xFB, 0x03, 0xFD, 0xA7, 0x89, 0x81, 0xB9,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngData)
	}))
	defer server.Close()

	checker := NewSiteChecker(DefaultCheckOptions())
	dataURL := checker.downloadFavicon(context.Background(), server.URL+"/favicon.png")
	if dataURL == "" {
		t.Fatal("expected data URL for valid favicon")
	}

	expectedPrefix := "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, expectedPrefix) {
		t.Fatalf("unexpected data URL prefix: %s", dataURL)
	}

	expectedPayload := base64.StdEncoding.EncodeToString(pngData)
	if payload := strings.TrimPrefix(dataURL, expectedPrefix); payload != expectedPayload {
		t.Fatalf("unexpected base64 payload: got %s want %s", payload, expectedPayload)
	}
}

func TestDownloadFaviconRejectsHTMLContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, "<html><body>not an image</body></html>")
	}))
	defer server.Close()

	checker := NewSiteChecker(DefaultCheckOptions())
	dataURL := checker.downloadFavicon(context.Background(), server.URL+"/favicon.ico")
	if dataURL != "" {
		t.Fatalf("expected empty data URL for non-image content, got %s", dataURL)
	}
}

func TestDownloadFaviconRejectsHTMLContentWithoutHeader(t *testing.T) {
	checker := NewSiteChecker(DefaultCheckOptions())
	checker.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("<html><body>not an image</body></html>")),
				Request:    req,
			}, nil
		}),
	}

	dataURL := checker.downloadFavicon(context.Background(), "http://example.com/favicon.ico")
	if dataURL != "" {
		t.Fatalf("expected empty data URL when header missing and content sniffing rejects, got %s", dataURL)
	}
}
