package sites

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

const siteNavigationProbeSecret = "Bearer site-navigation-probe-secret"

func siteWithProbeHeader() *sitecheck.SiteInfo {
	return &sitecheck.SiteInfo{
		SiteConfig: model.SiteConfig{
			Model:    model.Model{ID: 7},
			Host:     "example.com:443",
			Scheme:   "https",
			SiteName: "example",
			HealthCheckConfig: &model.HealthCheckConfig{
				Path:    "/health",
				Headers: map[string]string{"Authorization": siteNavigationProbeSecret},
			},
		},
		Name:   "example.com",
		Status: "online",
	}
}

func TestRedactSiteInfosHidesProbeHeadersWithoutTouchingTheChecker(t *testing.T) {
	original := siteWithProbeHeader()
	sites := []*sitecheck.SiteInfo{original, {Name: "no-health-check"}, nil}

	redacted := redactSiteInfos(sites)

	require.Len(t, redacted, 3)
	require.Nil(t, redacted[2])
	require.Equal(t, settings.RedactedSensitiveValue, redacted[0].HealthCheckConfig.Headers["Authorization"])
	require.Equal(t, "/health", redacted[0].HealthCheckConfig.Path)
	require.Equal(t, "example.com", redacted[0].Name)
	require.Equal(t, "online", redacted[0].Status)
	require.Equal(t, "no-health-check", redacted[1].Name)
	// The checker keeps probing with the real header, so its copy is unchanged.
	require.Equal(t, siteNavigationProbeSecret, original.HealthCheckConfig.Headers["Authorization"])
}

func receiveSiteUpdate(t *testing.T, messages <-chan ServerMessage) ServerMessage {
	t.Helper()
	for {
		select {
		case message := <-messages:
			if message.Type == MessageTypeUpdate {
				return message
			}
		case <-time.After(keepaliveTestTimeout):
			t.Fatal("no site navigation update received")
		}
	}
}

func TestSiteNavigationFeedHidesProbeHeadersFromReadOnlyTokens(t *testing.T) {
	readOnlyURL, _ := startSiteNavigationServerFor(t, true)
	adminURL, _ := startSiteNavigationServerFor(t, false)
	_, readOnlyMessages, _ := dialSiteNavigation(t, readOnlyURL, true)
	_, adminMessages, _ := dialSiteNavigation(t, adminURL, true)

	site := siteWithProbeHeader()
	wsManager.BroadcastUpdate([]*sitecheck.SiteInfo{site})

	readOnly := receiveSiteUpdate(t, readOnlyMessages)
	require.Len(t, readOnly.Data, 1)
	require.Equal(t, "example.com", readOnly.Data[0].Name)
	require.Equal(t, settings.RedactedSensitiveValue, readOnly.Data[0].HealthCheckConfig.Headers["Authorization"])

	admin := receiveSiteUpdate(t, adminMessages)
	require.Len(t, admin.Data, 1)
	require.Equal(t, siteNavigationProbeSecret, admin.Data[0].HealthCheckConfig.Headers["Authorization"])
	require.Equal(t, siteNavigationProbeSecret, site.HealthCheckConfig.Headers["Authorization"])
}
