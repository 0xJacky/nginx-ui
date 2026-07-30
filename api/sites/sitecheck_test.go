package sites

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateHealthCheckConfigPersistsFalseValuesAndJSONFields(t *testing.T) {
	originalDB := model.UseDB()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.SiteConfig{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.Use(db)
	t.Cleanup(func() { model.Use(originalDB) })

	config := &model.SiteConfig{
		SiteKey:            "example.conf|https://example.com:443",
		SiteName:           "example.conf",
		Host:               "example.com:443",
		HealthCheckEnabled: true,
		FollowRedirects:    true,
		CheckFavicon:       true,
	}
	if err := db.Create(config).Error; err != nil {
		t.Fatalf("failed to create site config: %v", err)
	}

	err = updateHealthCheckConfig(config, &updateHealthCheckRequest{
		HealthCheckEnabled: false,
		CheckInterval:      60,
		Timeout:            5,
		UserAgent:          "e2e-checker",
		MaxRedirects:       0,
		FollowRedirects:    false,
		CheckFavicon:       false,
		HealthCheckConfig: &model.HealthCheckConfig{
			TargetURL:      "https://example.com:8443",
			Protocol:       "https",
			Method:         "GET",
			Path:           "/ready",
			ExpectedStatus: []int{200},
		},
		HealthCheckAlert: &model.SiteHealthAlertConfig{
			Enabled:           true,
			StatusCodes:       []int{502, 503},
			NetworkErrors:     true,
			FailureThreshold:  2,
			RecoveryEnabled:   true,
			CooldownSeconds:   900,
			ExternalNotifyIDs: []uint64{7},
		},
	})
	if err != nil {
		t.Fatalf("updateHealthCheckConfig returned error: %v", err)
	}

	var saved model.SiteConfig
	if err := db.First(&saved, config.ID).Error; err != nil {
		t.Fatalf("failed to reload site config: %v", err)
	}
	if saved.HealthCheckEnabled || saved.FollowRedirects || saved.CheckFavicon {
		t.Fatalf("expected false values to persist, got enabled=%t redirects=%t favicon=%t", saved.HealthCheckEnabled, saved.FollowRedirects, saved.CheckFavicon)
	}
	if saved.HealthCheckConfig == nil || saved.HealthCheckConfig.TargetURL != "https://example.com:8443" {
		t.Fatalf("expected health check JSON to persist, got %#v", saved.HealthCheckConfig)
	}
	if saved.HealthCheckAlert == nil || len(saved.HealthCheckAlert.ExternalNotifyIDs) != 1 || saved.HealthCheckAlert.ExternalNotifyIDs[0] != 7 {
		t.Fatalf("expected alert JSON to persist, got %#v", saved.HealthCheckAlert)
	}
}

func TestSyncHealthCheckUsesStableKeyAndPreservesLocalNotifierIDs(t *testing.T) {
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

	existing := &model.SiteConfig{
		SiteKey:            "example.conf|https://example.com:443",
		SiteName:           "example.conf",
		Host:               "example.com:443",
		HealthCheckEnabled: true,
		HealthCheckAlert: &model.SiteHealthAlertConfig{
			Enabled:           true,
			ExternalNotifyIDs: []uint64{9},
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("failed to create existing site config: %v", err)
	}

	payload := healthCheckSyncPayload{
		SiteKey:    existing.SiteKey,
		SiteName:   existing.SiteName,
		Host:       existing.Host,
		Port:       443,
		Scheme:     "https",
		DisplayURL: "https://example.com",
		Config: updateHealthCheckRequest{
			HealthCheckEnabled: false,
			CheckInterval:      60,
			Timeout:            5,
			UserAgent:          "sync-checker",
			MaxRedirects:       0,
			FollowRedirects:    false,
			CheckFavicon:       false,
			HealthCheckConfig: &model.HealthCheckConfig{
				TargetURL:      "https://127.0.0.1:8443",
				Protocol:       "https",
				Method:         "GET",
				Path:           "/ready",
				ExpectedStatus: []int{200},
			},
			HealthCheckAlert: &model.SiteHealthAlertConfig{
				Enabled:          true,
				StatusCodes:      []int{502},
				FailureThreshold: 2,
				RecoveryEnabled:  true,
				CooldownSeconds:  900,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal sync payload: %v", err)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/site_navigation/health_check/sync", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	SyncHealthCheck(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var count int64
	if err := db.Model(&model.SiteConfig{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count site configs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected stable-key sync to update one record, got %d records", count)
	}
	var saved model.SiteConfig
	if err := db.First(&saved, existing.ID).Error; err != nil {
		t.Fatalf("failed to reload synchronized config: %v", err)
	}
	if saved.HealthCheckEnabled || saved.FollowRedirects || saved.CheckFavicon {
		t.Fatalf("expected synchronized false values to persist, got enabled=%t redirects=%t favicon=%t", saved.HealthCheckEnabled, saved.FollowRedirects, saved.CheckFavicon)
	}
	if saved.HealthCheckConfig == nil || saved.HealthCheckConfig.TargetURL != "https://127.0.0.1:8443" {
		t.Fatalf("expected synchronized target URL to persist, got %#v", saved.HealthCheckConfig)
	}
	if saved.HealthCheckAlert == nil || len(saved.HealthCheckAlert.ExternalNotifyIDs) != 1 || saved.HealthCheckAlert.ExternalNotifyIDs[0] != 9 {
		t.Fatalf("expected node-local notifier IDs to be preserved, got %#v", saved.HealthCheckAlert)
	}
}
