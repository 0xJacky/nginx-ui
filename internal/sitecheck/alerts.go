package sitecheck

import (
	"slices"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

var siteHealthAlertNow = time.Now
var siteHealthAlertLocks sync.Map

func lockSiteHealthAlert(siteKey string) func() {
	lockValue, _ := siteHealthAlertLocks.LoadOrStore(siteKey, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func isAlertFailure(policy *model.SiteHealthAlertConfig, info *SiteInfo) bool {
	if policy == nil || info == nil {
		return false
	}
	if info.StatusCode > 0 && slices.Contains(policy.StatusCodes, info.StatusCode) {
		return true
	}
	return policy.NetworkErrors && info.StatusCode == 0 &&
		(info.Status == StatusOffline || info.Status == StatusError || info.Error != "")
}

func alertDetails(config *model.SiteConfig, info *SiteInfo, failureCount int) map[string]any {
	targetURL := config.GetURL()
	if config.HealthCheckConfig != nil && config.HealthCheckConfig.TargetURL != "" {
		targetURL = config.HealthCheckConfig.TargetURL
	}
	return map[string]any{
		"node":          settings.NodeSettings.Name,
		"site":          config.SiteName,
		"target":        targetURL,
		"status_code":   info.StatusCode,
		"error":         info.Error,
		"failure_count": failureCount,
	}
}

func evaluateSiteHealthAlert(config *model.SiteConfig, info *SiteInfo) {
	policy := config.HealthCheckAlert
	if policy == nil || !policy.Enabled || !info.EffectiveHealthCheckEnabled || config.SiteKey == "" {
		return
	}
	unlock := lockSiteHealthAlert(config.SiteKey)
	defer unlock()

	db := model.UseDB()
	if db == nil {
		return
	}

	state := &model.SiteHealthAlertState{}
	err := db.Where("site_key = ?", config.SiteKey).First(state).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logger.Errorf("Failed to load health alert state for %s: %v", config.SiteKey, err)
		return
	}
	if err == gorm.ErrRecordNotFound {
		state.SiteKey = config.SiteKey
	}

	now := siteHealthAlertNow()
	if isAlertFailure(policy, info) {
		state.ConsecutiveFailures++
		threshold := policy.FailureThreshold
		if threshold < 1 {
			threshold = 1
		}

		shouldNotify := state.ConsecutiveFailures >= threshold && !state.FailureNotified
		if state.FailureNotified && policy.CooldownSeconds > 0 && state.LastNotifiedAt != nil {
			shouldNotify = now.Sub(*state.LastNotifiedAt) >= time.Duration(policy.CooldownSeconds)*time.Second
		}
		if shouldNotify {
			notification.WarningTo(
				"Site Health Check Failed",
				"Site %{site} on node %{node} failed its health check for %{failure_count} consecutive attempts: %{error}",
				alertDetails(config, info, state.ConsecutiveFailures),
				policy.ExternalNotifyIDs,
			)
			state.FailureNotified = true
			state.LastNotifiedAt = &now
		}
		state.LastStatus = info.Status
	} else if info.Status == StatusOnline {
		if state.FailureNotified && policy.RecoveryEnabled {
			notification.SuccessTo(
				"Site Health Check Recovered",
				"Site %{site} on node %{node} recovered. Target: %{target}",
				alertDetails(config, info, 0),
				policy.ExternalNotifyIDs,
			)
		}
		state.ConsecutiveFailures = 0
		state.FailureNotified = false
		state.LastNotifiedAt = nil
		state.LastStatus = info.Status
	} else {
		// A failure outside the selected policy is not a recovery. Reset only a
		// pending threshold and retain any notified outage until the site is
		// actually healthy again.
		state.ConsecutiveFailures = 0
		state.LastStatus = info.Status
	}

	if state.ID == 0 {
		err = db.Create(state).Error
	} else {
		err = db.Save(state).Error
	}
	if err != nil {
		logger.Errorf("Failed to persist health alert state for %s: %v", config.SiteKey, err)
	}
}
