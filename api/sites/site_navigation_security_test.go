package sites

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

	redacted := redactSiteInfos(sites, hideHealthCheckSecrets)

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

// An api:write token does not read the form values of a plugin probe kind
// either, but keeps every other probe setting; the web UI's view is untouched.
func TestRedactSiteInfosHidesOnlyProbeConfigFromWriteTokens(t *testing.T) {
	original := siteWithProbeHeader()
	sites := []*sitecheck.SiteInfo{original}

	redacted := redactSiteInfos(sites, hideProbeConfigValues)

	require.Len(t, redacted, 1)
	require.Equal(t, map[string]string{"password": settings.RedactedSensitiveValue}, redacted[0].ProbeConfig)
	require.Equal(t, siteNavigationProbeSecret, redacted[0].HealthCheckConfig.Headers["Authorization"])
	require.Equal(t, original.HealthCheckConfig.Body, redacted[0].HealthCheckConfig.Body)
	require.Equal(t, original.HealthCheckConfig.TargetURL, redacted[0].HealthCheckConfig.TargetURL)
	require.Equal(t, siteNavigationProbeFormSecret, original.ProbeConfig["password"])

	shown := redactSiteInfos(sites, showHealthCheckSecrets)
	require.Equal(t, siteNavigationProbeFormSecret, shown[0].ProbeConfig["password"])
	require.Equal(t, siteNavigationProbeSecret, shown[0].HealthCheckConfig.Headers["Authorization"])
}

// The form values of a plugin probe kind may be credentials, so the health
// check endpoint hides them from every service token, api:write included, and
// keeps the keys. The other probe secrets stay readable for api:write and are
// hidden from api:read only. The web UI still reads every value, and its read,
// edit, save cycle keeps the stored values.
func TestGetHealthCheckRedactsProbeConfigForEveryServiceToken(t *testing.T) {
	registerStaticProbeSource()
	db := openSiteConfigDB(t)
	stored := map[string]string{"port": "6379", "password": siteNavigationProbeFormSecret}
	config := &model.SiteConfig{SiteKey: "probe.conf|https://probe.example:443", Host: "probe.example:443"}
	require.NoError(t, db.Create(config).Error)
	request := probeRequest("plugin:api-test", stored)
	request.HealthCheckConfig.Headers = map[string]string{"Authorization": siteNavigationProbeSecret}
	require.NoError(t, updateHealthCheckConfig(config, request))

	gin.SetMode(gin.TestMode)
	placeholders := map[string]string{
		"port":     settings.RedactedSensitiveValue,
		"password": settings.RedactedSensitiveValue,
	}

	readOnly := readHealthCheck(t, config.ID, model.APITokenScopeRead)
	require.Equal(t, "plugin:api-test", readOnly.ProbeKind)
	require.Equal(t, placeholders, readOnly.ProbeConfig)
	require.Equal(t, settings.RedactedSensitiveValue, readOnly.HealthCheckConfig.Headers["Authorization"])

	writer := readHealthCheck(t, config.ID, model.APITokenScopeRead, model.APITokenScopeWrite)
	require.Equal(t, "plugin:api-test", writer.ProbeKind)
	require.Equal(t, placeholders, writer.ProbeConfig)
	require.Equal(t, siteNavigationProbeSecret, writer.HealthCheckConfig.Headers["Authorization"],
		"api:write keeps reading the other probe secrets")

	// The web UI saves what it read back unchanged.
	admin := readHealthCheck(t, config.ID)
	require.Equal(t, stored, admin.ProbeConfig)
	require.Equal(t, siteNavigationProbeSecret, admin.HealthCheckConfig.Headers["Authorization"])
	resave := probeRequest(admin.ProbeKind, admin.ProbeConfig)
	resave.HealthCheckConfig = admin.HealthCheckConfig
	require.NoError(t, updateHealthCheckConfig(config, resave))
	var saved model.SiteConfig
	require.NoError(t, db.First(&saved, config.ID).Error)
	require.Equal(t, stored, saved.ProbeConfig)
}

// readHealthCheck reads a health check as a service token with scopes, or as
// the web UI without them.
func readHealthCheck(t *testing.T, id uint64, scopes ...string) model.SiteConfig {
	t.Helper()
	router := gin.New()
	router.GET("/site_navigation/health_check/:id", withServiceToken(scopes), GetHealthCheck)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/site_navigation/health_check/%d", id), nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var got model.SiteConfig
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	return got
}

// openHealthCheckSaveDB opens a site config database that UpdateHealthCheck
// can also look the synchronized nodes of a site up in.
func openHealthCheckSaveDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openSiteConfigDB(t)
	require.NoError(t, db.AutoMigrate(&model.Site{}, &model.Namespace{}, &model.Node{}))
	originalSite, originalNode := query.Site, query.Node
	testQuery := query.Use(db)
	query.Site, query.Node = &testQuery.Site, &testQuery.Node
	t.Cleanup(func() { query.Site, query.Node = originalSite, originalNode })
	return db
}

// saveHealthCheck sends body to UpdateHealthCheck as a service token with
// scopes, or as the web UI without them.
func saveHealthCheck(t *testing.T, id uint64, body any, scopes ...string) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	router := gin.New()
	router.POST("/site_navigation/health_check/:id", withServiceToken(scopes), UpdateHealthCheck)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/site_navigation/health_check/%d", id), bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func withServiceToken(scopes []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if scopes != nil {
			c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{Scopes: scopes})
		}
	}
}

// writeRequestFrom turns a health check read into the body an automation
// sends back after changing the check interval.
func writeRequestFrom(read model.SiteConfig) map[string]any {
	return map[string]any{
		"health_check_enabled": read.HealthCheckEnabled,
		"check_interval":       read.CheckInterval + 60,
		"timeout":              read.Timeout,
		"health_check_config":  read.HealthCheckConfig,
		"probe_kind":           read.ProbeKind,
		"probe_config":         read.ProbeConfig,
	}
}

// An api:write automation reads a health check, changes something else and
// saves the record back. The placeholders it read, or a probe_config it left
// out, keep the stored form values instead of overwriting them. The
// placeholder never keeps a stored value once the probe would use it
// differently (another kind, another target, other keys) or when it is mixed
// with other values.
func TestUpdateHealthCheckKeepsStoredProbeConfigForWriteTokenRoundTrip(t *testing.T) {
	registerStaticProbeSource()
	db := openHealthCheckSaveDB(t)
	stored := map[string]string{"port": "6379", "password": siteNavigationProbeFormSecret}
	config := &model.SiteConfig{SiteKey: "probe.conf|https://probe.example:443", Host: "probe.example:443"}
	require.NoError(t, db.Create(config).Error)
	request := probeRequest("plugin:api-test", stored)
	request.HealthCheckConfig.TargetURL = "https://10.0.0.2:8443"
	require.NoError(t, updateHealthCheckConfig(config, request))
	gin.SetMode(gin.TestMode)
	writeScopes := []string{model.APITokenScopeRead, model.APITokenScopeWrite}

	storedConfig := func(t *testing.T) model.SiteConfig {
		t.Helper()
		var saved model.SiteConfig
		require.NoError(t, db.First(&saved, config.ID).Error)
		return saved
	}

	read := readHealthCheck(t, config.ID, writeScopes...)
	require.Equal(t, settings.RedactedSensitiveValue, read.ProbeConfig["password"])

	t.Run("placeholders keep the stored values", func(t *testing.T) {
		recorder := saveHealthCheck(t, config.ID, writeRequestFrom(read), writeScopes...)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		saved := storedConfig(t)
		require.Equal(t, stored, saved.ProbeConfig)
		require.Equal(t, read.CheckInterval+60, saved.CheckInterval, "the other change is saved")
	})

	t.Run("an omitted probe_config keeps the stored values", func(t *testing.T) {
		body := writeRequestFrom(read)
		delete(body, "probe_config")
		recorder := saveHealthCheck(t, config.ID, body, writeScopes...)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, stored, storedConfig(t).ProbeConfig)
	})

	refused := map[string]func(body map[string]any){
		"another probe target": func(body map[string]any) {
			changed := *read.HealthCheckConfig
			changed.TargetURL = "https://attacker.example:6379"
			body["health_check_config"] = &changed
		},
		"another value next to a placeholder": func(body map[string]any) {
			body["probe_config"] = map[string]string{"port": "16379", "password": settings.RedactedSensitiveValue}
		},
		// The placeholder next to a value is refused whether or not the value
		// is the stored one, so the answer cannot confirm a guess.
		"the stored value next to a placeholder": func(body map[string]any) {
			body["probe_config"] = map[string]string{"port": "6379", "password": settings.RedactedSensitiveValue}
		},
		"another key next to a placeholder": func(body map[string]any) {
			body["probe_config"] = map[string]string{"port": settings.RedactedSensitiveValue, "password": settings.RedactedSensitiveValue, "host": "attacker.example"}
		},
		"a key left out next to a placeholder": func(body map[string]any) {
			body["probe_config"] = map[string]string{"password": settings.RedactedSensitiveValue}
		},
		"another probe kind": func(body map[string]any) {
			body["probe_kind"] = "plugin:other"
		},
	}
	for name, change := range refused {
		t.Run(name+" needs every value", func(t *testing.T) {
			body := writeRequestFrom(read)
			change(body)
			recorder := saveHealthCheck(t, config.ID, body, writeScopes...)
			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), "send every value")
			require.Equal(t, stored, storedConfig(t).ProbeConfig)
		})
	}

	t.Run("an omitted probe_config with another target is refused", func(t *testing.T) {
		body := writeRequestFrom(read)
		delete(body, "probe_config")
		changed := *read.HealthCheckConfig
		changed.TargetURL = "https://attacker.example:6379"
		body["health_check_config"] = &changed
		recorder := saveHealthCheck(t, config.ID, body, writeScopes...)
		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), "probe_config is required")
		require.Equal(t, stored, storedConfig(t).ProbeConfig)
	})

	t.Run("new values without a placeholder are saved", func(t *testing.T) {
		body := writeRequestFrom(read)
		changed := *read.HealthCheckConfig
		changed.TargetURL = "https://10.0.0.3:8443"
		body["health_check_config"] = &changed
		replaced := map[string]string{"port": "6380", "password": "rotated"}
		body["probe_config"] = replaced
		recorder := saveHealthCheck(t, config.ID, body, writeScopes...)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, replaced, storedConfig(t).ProbeConfig)
	})
}

// The web UI keeps its full round trip: it reads every value and saves them
// back, including a new target with the stored values.
func TestUpdateHealthCheckWebUIRoundTripKeepsFullValues(t *testing.T) {
	registerStaticProbeSource()
	db := openHealthCheckSaveDB(t)
	stored := map[string]string{"port": "6379", "password": siteNavigationProbeFormSecret}
	config := &model.SiteConfig{SiteKey: "probe.conf|https://probe.example:443", Host: "probe.example:443"}
	require.NoError(t, db.Create(config).Error)
	require.NoError(t, updateHealthCheckConfig(config, probeRequest("plugin:api-test", stored)))
	gin.SetMode(gin.TestMode)

	read := readHealthCheck(t, config.ID)
	require.Equal(t, stored, read.ProbeConfig)
	body := writeRequestFrom(read)
	changed := *read.HealthCheckConfig
	changed.TargetURL = "https://10.0.0.4:8443"
	body["health_check_config"] = &changed
	recorder := saveHealthCheck(t, config.ID, body)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var saved model.SiteConfig
	require.NoError(t, db.First(&saved, config.ID).Error)
	require.Equal(t, stored, saved.ProbeConfig)
	require.Equal(t, "https://10.0.0.4:8443", saved.HealthCheckConfig.TargetURL)
}

// Testing a probe with the placeholders an api:write token read runs it with
// the stored values, under the same conditions as a save.
func TestTestHealthCheckUsesStoredProbeConfigForPlaceholders(t *testing.T) {
	source := registerStaticProbeSource()
	source.reset()
	db := openSiteConfigDB(t)
	config := &model.SiteConfig{SiteKey: "api.conf|https://api.example:443", Host: "api.example:443", Scheme: "https", Timeout: 5}
	require.NoError(t, db.Create(config).Error)
	request := probeRequest("plugin:api-test", map[string]string{"port": "22", "user": "admin"})
	request.HealthCheckConfig.TargetURL = "https://10.0.0.2:8443"
	require.NoError(t, updateHealthCheckConfig(config, request))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/site_navigation/test_health_check/:id",
		withServiceToken([]string{model.APITokenScopeRead, model.APITokenScopeWrite}), TestHealthCheck)
	send := func(target string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"config":{"protocol":"https","target_url":%q},"probe_kind":"plugin:api-test","probe_config":{"port":%q,"user":%q}}`,
			target, settings.RedactedSensitiveValue, settings.RedactedSensitiveValue)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/site_navigation/test_health_check/%d", config.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		return recorder
	}

	recorder := send("https://10.0.0.2:8443")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	recorder = send("https://attacker.example:22")
	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())

	// A value next to the placeholder is refused, right guess or not.
	for _, guess := range []string{"admin", "root"} {
		body := fmt.Sprintf(`{"config":{"protocol":"https","target_url":"https://10.0.0.2:8443"},"probe_kind":"plugin:api-test","probe_config":{"port":%q,"user":%q}}`,
			settings.RedactedSensitiveValue, guess)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/site_navigation/test_health_check/%d", config.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	require.Equal(t, []string{"https://10.0.0.2:8443|22"}, source.targets)
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

func TestSiteNavigationFeedHidesProbeSecretsFromServiceTokens(t *testing.T) {
	readOnlyURL, _ := startSiteNavigationServerFor(t, hideHealthCheckSecrets)
	writerURL, _ := startSiteNavigationServerFor(t, hideProbeConfigValues)
	adminURL, _ := startSiteNavigationServerFor(t, showHealthCheckSecrets)
	_, readOnlyMessages, _ := dialSiteNavigation(t, readOnlyURL, true)
	_, writerMessages, _ := dialSiteNavigation(t, writerURL, true)
	_, adminMessages, _ := dialSiteNavigation(t, adminURL, true)

	site := siteWithProbeHeader()
	wsManager.BroadcastUpdate([]*sitecheck.SiteInfo{site})

	readOnly := receiveSiteUpdate(t, readOnlyMessages)
	require.Len(t, readOnly.Data, 1)
	require.Equal(t, "example.com", readOnly.Data[0].Name)
	require.Equal(t, settings.RedactedSensitiveValue, readOnly.Data[0].HealthCheckConfig.Headers["Authorization"])

	// api:write keeps the header values.
	writer := receiveSiteUpdate(t, writerMessages)
	require.Len(t, writer.Data, 1)
	require.Equal(t, siteNavigationProbeSecret, writer.Data[0].HealthCheckConfig.Headers["Authorization"])

	admin := receiveSiteUpdate(t, adminMessages)
	require.Len(t, admin.Data, 1)
	require.Equal(t, siteNavigationProbeSecret, admin.Data[0].HealthCheckConfig.Headers["Authorization"])

	// The feed never carries plugin form values; the editor reads them from
	// the health check endpoint.
	for _, message := range []ServerMessage{readOnly, writer, admin} {
		require.Nil(t, message.Data[0].ProbeConfig)
	}
	require.Equal(t, siteNavigationProbeSecret, site.HealthCheckConfig.Headers["Authorization"])
}
