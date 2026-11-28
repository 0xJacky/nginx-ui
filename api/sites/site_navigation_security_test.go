package sites

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const siteNavigationProbeSecret = "Bearer site-navigation-probe-secret"

const siteNavigationProbeFormSecret = "site-navigation-probe-form-secret"

func siteWithProbeHeader() *sitecheck.SiteInfo {
	return &sitecheck.SiteInfo{
		SiteConfig: model.SiteConfig{
			Model:       model.Model{ID: 7},
			Host:        "example.com:443",
			Scheme:      "https",
			SiteName:    "example",
			ProbeKind:   "plugin:api-test",
			ProbeConfig: map[string]string{"password": siteNavigationProbeFormSecret},
			HealthCheckConfig: &model.HealthCheckConfig{
				Path:      "/health",
				Headers:   map[string]string{"Authorization": siteNavigationProbeSecret},
				Body:      `{"password":"probe-body-secret"}`,
				TargetURL: "https://probe:probe-url-secret@example.com/health?token=probe-query-secret",
				ClickURL:  "https://example.com/status",
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
	require.Equal(t, settings.RedactedSensitiveValue, redacted[0].HealthCheckConfig.Body)
	require.NotContains(t, redacted[0].HealthCheckConfig.TargetURL, "probe-url-secret")
	require.NotContains(t, redacted[0].HealthCheckConfig.TargetURL, "probe-query-secret")
	require.Contains(t, redacted[0].HealthCheckConfig.TargetURL, "example.com/health")
	require.Equal(t, "https://example.com/status", redacted[0].HealthCheckConfig.ClickURL)
	require.Equal(t, map[string]string{"password": settings.RedactedSensitiveValue}, redacted[0].ProbeConfig)
	require.Equal(t, "/health", redacted[0].HealthCheckConfig.Path)
	require.Equal(t, "example.com", redacted[0].Name)
	require.Equal(t, "online", redacted[0].Status)
	require.Equal(t, "no-health-check", redacted[1].Name)
	// The checker keeps probing with the real settings, so its copy is unchanged.
	require.Equal(t, siteNavigationProbeSecret, original.HealthCheckConfig.Headers["Authorization"])
	require.Contains(t, original.HealthCheckConfig.Body, "probe-body-secret")
	require.Contains(t, original.HealthCheckConfig.TargetURL, "probe-url-secret")
	require.Equal(t, siteNavigationProbeFormSecret, original.ProbeConfig["password"])
}

// GHSA-fpv9-q72v-fgrc: the form values of a plugin probe kind may be
// credentials, so the health check endpoint hides them from a read-only
// service token and keeps the keys. An api:write token and the web UI still
// read them in full, and the editor's read, edit, save cycle keeps the stored
// values.
func TestGetHealthCheckRedactsProbeConfigForReadOnlyTokens(t *testing.T) {
	registerStaticProbeSource()
	db := openSiteConfigDB(t)
	stored := map[string]string{"port": "6379", "password": siteNavigationProbeFormSecret}
	config := &model.SiteConfig{SiteKey: "probe.conf|https://probe.example:443", Host: "probe.example:443"}
	require.NoError(t, db.Create(config).Error)
	require.NoError(t, updateHealthCheckConfig(config, probeRequest("plugin:api-test", stored)))

	gin.SetMode(gin.TestMode)
	read := func(t *testing.T, scopes ...string) model.SiteConfig {
		t.Helper()
		router := gin.New()
		router.GET("/site_navigation/health_check/:id", func(c *gin.Context) {
			if scopes != nil {
				c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{Scopes: scopes})
			}
		}, GetHealthCheck)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/site_navigation/health_check/%d", config.ID), nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var got model.SiteConfig
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
		return got
	}

	readOnly := read(t, model.APITokenScopeRead)
	require.Equal(t, "plugin:api-test", readOnly.ProbeKind)
	require.Equal(t, map[string]string{
		"port":     settings.RedactedSensitiveValue,
		"password": settings.RedactedSensitiveValue,
	}, readOnly.ProbeConfig)

	require.Equal(t, stored, read(t, model.APITokenScopeRead, model.APITokenScopeWrite).ProbeConfig)

	// The web UI saves what it read back unchanged.
	admin := read(t)
	require.Equal(t, stored, admin.ProbeConfig)
	resave := probeRequest(admin.ProbeKind, admin.ProbeConfig)
	resave.HealthCheckConfig = admin.HealthCheckConfig
	require.NoError(t, updateHealthCheckConfig(config, resave))
	var saved model.SiteConfig
	require.NoError(t, db.First(&saved, config.ID).Error)
	require.Equal(t, stored, saved.ProbeConfig)
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
