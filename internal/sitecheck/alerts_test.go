package sitecheck

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSiteHealthAlertTransitionsAreDeduplicated(t *testing.T) {
	originalDB := model.UseDB()
	originalNotification := query.Notification
	originalExternalNotify := query.ExternalNotify
	originalNow := siteHealthAlertNow

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.Notification{}, &model.ExternalNotify{}, &model.SiteHealthAlertState{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.Use(db)
	testQuery := query.Use(db)
	query.Notification = &testQuery.Notification
	query.ExternalNotify = &testQuery.ExternalNotify

	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	siteHealthAlertNow = func() time.Time { return now }
	t.Cleanup(func() {
		model.Use(originalDB)
		query.Notification = originalNotification
		query.ExternalNotify = originalExternalNotify
		siteHealthAlertNow = originalNow
	})

	config := &model.SiteConfig{
		SiteKey:  "example.conf|https://example.com:443",
		SiteName: "example.conf",
		HealthCheckAlert: &model.SiteHealthAlertConfig{
			Enabled:          true,
			StatusCodes:      []int{httpStatusBadGateway},
			FailureThreshold: 2,
			RecoveryEnabled:  true,
			CooldownSeconds:  900,
		},
	}
	failure := &SiteInfo{
		Status:                      StatusOffline,
		StatusCode:                  httpStatusBadGateway,
		Error:                       "Unexpected status code: 502",
		EffectiveHealthCheckEnabled: true,
	}

	evaluateSiteHealthAlert(config, failure)
	assertNotificationCount(t, db, 0)
	evaluateSiteHealthAlert(config, failure)
	assertNotificationCount(t, db, 1)
	evaluateSiteHealthAlert(config, failure)
	assertNotificationCount(t, db, 1)

	unselectedFailure := &SiteInfo{
		Status:                      StatusError,
		StatusCode:                  500,
		Error:                       "Unexpected status code: 500",
		EffectiveHealthCheckEnabled: true,
	}
	evaluateSiteHealthAlert(config, unselectedFailure)
	assertNotificationCount(t, db, 1)

	recovery := &SiteInfo{
		Status:                      StatusOnline,
		StatusCode:                  200,
		EffectiveHealthCheckEnabled: true,
	}
	evaluateSiteHealthAlert(config, recovery)
	assertNotificationCount(t, db, 2)
	evaluateSiteHealthAlert(config, recovery)
	assertNotificationCount(t, db, 2)

	var notifications []model.Notification
	if err := db.Order("id asc").Find(&notifications).Error; err != nil {
		t.Fatalf("failed to load notifications: %v", err)
	}
	if notifications[0].Type != model.NotificationWarning || notifications[1].Type != model.NotificationSuccess {
		t.Fatalf("expected warning then recovery notifications, got %v then %v", notifications[0].Type, notifications[1].Type)
	}
}

func TestSiteHealthAlertConcurrentFailuresNotifyOnce(t *testing.T) {
	db, cleanup := setupSiteHealthAlertTest(t)
	defer cleanup()

	config := &model.SiteConfig{
		SiteKey:  "concurrent.conf|https://example.com:443",
		SiteName: "concurrent.conf",
		HealthCheckAlert: &model.SiteHealthAlertConfig{
			Enabled:          true,
			StatusCodes:      []int{httpStatusBadGateway},
			FailureThreshold: 1,
			CooldownSeconds:  900,
		},
	}
	failure := &SiteInfo{
		Status:                      StatusError,
		StatusCode:                  httpStatusBadGateway,
		Error:                       "Unexpected status code: 502",
		EffectiveHealthCheckEnabled: true,
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			evaluateSiteHealthAlert(config, failure)
		}()
	}
	wg.Wait()
	assertNotificationCount(t, db, 1)
}

func setupSiteHealthAlertTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	originalDB := model.UseDB()
	originalNotification := query.Notification
	originalExternalNotify := query.ExternalNotify

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.Notification{}, &model.ExternalNotify{}, &model.SiteHealthAlertState{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.Use(db)
	testQuery := query.Use(db)
	query.Notification = &testQuery.Notification
	query.ExternalNotify = &testQuery.ExternalNotify

	return db, func() {
		model.Use(originalDB)
		query.Notification = originalNotification
		query.ExternalNotify = originalExternalNotify
	}
}

const httpStatusBadGateway = 502

func assertNotificationCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Notification{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count notifications: %v", err)
	}
	if count != want {
		t.Fatalf("expected %d notifications, got %d", want, count)
	}
}
