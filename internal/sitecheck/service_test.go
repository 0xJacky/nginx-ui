package sitecheck

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestSetUpdateCallbackBeforeInit(t *testing.T) {
	globalServiceMu.Lock()
	previousService := globalService
	previousCallback := globalUpdateCallback
	globalService = nil
	globalUpdateCallback = nil
	globalServiceMu.Unlock()

	previousEnabled := settings.SiteCheckSettings.Enabled
	settings.SiteCheckSettings.Enabled = false

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		settings.SiteCheckSettings.Enabled = previousEnabled

		globalServiceMu.Lock()
		globalService = previousService
		globalUpdateCallback = previousCallback
		globalServiceMu.Unlock()
	})

	callbackCalled := false
	SetUpdateCallback(func([]*SiteInfo) {
		callbackCalled = true
	})
	Init(ctx)

	service := GetService()
	if service == nil {
		t.Fatal("expected site check service to be initialized")
	}

	service.checker.mu.RLock()
	updateCallback := service.checker.onUpdateCallback
	service.checker.mu.RUnlock()
	if updateCallback == nil {
		t.Fatal("expected callback registered before initialization to be retained")
	}

	updateCallback(nil)
	if !callbackCalled {
		t.Fatal("expected retained callback to be callable after initialization")
	}
}
