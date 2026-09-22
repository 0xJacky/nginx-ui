package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// staticProbeSource serves one kind and answers every check with up.
type staticProbeSource struct {
	mu      sync.Mutex
	targets []string
}

func (s *staticProbeSource) ProbeKinds() []sitecheck.ProbeKind {
	return []sitecheck.ProbeKind{{Kind: "plugin:api-test", Name: "API test", Fields: []sitecheck.ProbeField{
		{Key: "port", Type: "number", DisplayName: "Port", Required: true},
	}}}
}

func (s *staticProbeSource) Probe(_ context.Context, _ string, req sitecheck.ProbeRequest) (sitecheck.ProbeOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets = append(s.targets, req.Target+"|"+req.Config["port"])
	return sitecheck.ProbeOutcome{Status: sitecheck.ProbeUp}, nil
}

var registerStaticProbeSource = sync.OnceValue(func() *staticProbeSource {
	source := &staticProbeSource{}
	sitecheck.RegisterProbeSource(source)
	return source
})

func openSiteConfigDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.UseDB()
	originalSiteConfig := query.SiteConfig
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.SiteConfig{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.Use(db)
	testQuery := query.Use(db)
	query.SiteConfig = &testQuery.SiteConfig
	t.Cleanup(func() {
		model.Use(originalDB)
		query.SiteConfig = originalSiteConfig
	})
	return db
}

func probeRequest(kind string, config map[string]string) *updateHealthCheckRequest {
	return &updateHealthCheckRequest{
		HealthCheckEnabled: true,
		CheckInterval:      60,
		Timeout:            5,
		HealthCheckConfig:  &model.HealthCheckConfig{Protocol: "https", Method: "GET", Path: "/"},
		ProbeKind:          kind,
		ProbeConfig:        config,
	}
}

func TestUpdateHealthCheckConfigStoresTheProbeKind(t *testing.T) {
	db := openSiteConfigDB(t)
	config := &model.SiteConfig{SiteKey: "probe.conf|https://probe.example:443", Host: "probe.example:443"}
	if err := db.Create(config).Error; err != nil {
		t.Fatal(err)
	}

	if err := updateHealthCheckConfig(config, probeRequest("plugin:api-test", map[string]string{"port": "22"})); err != nil {
		t.Fatalf("updateHealthCheckConfig: %v", err)
	}
	var saved model.SiteConfig
	if err := db.First(&saved, config.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ProbeKind != "plugin:api-test" || saved.ProbeConfig["port"] != "22" {
		t.Fatalf("saved probe = %q %v", saved.ProbeKind, saved.ProbeConfig)
	}

	// Going back to the built-in check clears both columns, so the JSON of
	// the site looks exactly like before probe kinds existed.
	if err := updateHealthCheckConfig(config, probeRequest(sitecheck.ProbeKindHTTP, map[string]string{"port": "22"})); err != nil {
		t.Fatalf("updateHealthCheckConfig: %v", err)
	}
	saved = model.SiteConfig{}
	if err := db.First(&saved, config.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ProbeKind != "" || saved.ProbeConfig != nil {
		t.Fatalf("saved probe = %q %v, want the built-in check", saved.ProbeKind, saved.ProbeConfig)
	}
	encoded, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "probe_") {
		t.Fatalf("a built-in check leaks probe fields into the response: %s", encoded)
	}

	if err := validateHealthCheckRequest(probeRequest("Plugin: Bad!", nil)); err == nil {
		t.Fatal("a malformed probe kind was accepted")
	}
}

func TestGetProbeKindsAndTestHealthCheckWithAProbeKind(t *testing.T) {
	source := registerStaticProbeSource()
	db := openSiteConfigDB(t)
	config := &model.SiteConfig{SiteKey: "api.conf|https://api.example:443", Host: "api.example:443", Scheme: "https", Timeout: 5}
	if err := db.Create(config).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/site_navigation/probe_kinds", GetProbeKinds)
	router.POST("/site_navigation/test_health_check/:id", TestHealthCheck)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/site_navigation/probe_kinds", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"kind":"plugin:api-test"`) {
		t.Fatalf("probe kinds = %d %s", recorder.Code, recorder.Body.String())
	}

	body := `{"config":{"protocol":"https","target_url":"https://10.0.0.2:8443"},"probe_kind":"plugin:api-test","probe_config":{"port":"22"}}`
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/site_navigation/test_health_check/%d", config.ID), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("test = %d %s", recorder.Code, recorder.Body.String())
	}
	var result struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Status != sitecheck.StatusOnline {
		t.Fatalf("result = %+v", result)
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.targets) != 1 || source.targets[0] != "https://10.0.0.2:8443|22" {
		t.Fatalf("probed = %v", source.targets)
	}
}
