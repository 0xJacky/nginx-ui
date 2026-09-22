package sitecheck

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

func calculateCertDaysRemaining(state *tls.ConnectionState) *int64 {
	if state == nil || len(state.PeerCertificates) == 0 {
		return nil
	}

	remaining := state.PeerCertificates[0].NotAfter.Sub(time.Now())
	day := 24 * time.Hour

	var days int64
	if remaining >= 0 {
		days = int64((remaining + day - time.Nanosecond) / day)
	} else {
		days = -int64(((-remaining) + day - time.Nanosecond) / day)
	}

	return &days
}

// Site config cache with expiration
var (
	siteConfigCache   = make(map[string]*siteConfigCacheEntry)
	siteConfigMutex   sync.RWMutex
	cacheExpiry       = 5 * time.Minute // Cache entries expire after 5 minutes
	lastBatchLoad     time.Time
	linkReconcileOnce sync.Once
)

type siteConfigCacheEntry struct {
	config    *model.SiteConfig
	expiresAt time.Time
}

type SiteChecker struct {
	sites            map[string]*SiteInfo
	mu               sync.RWMutex
	options          CheckOptions
	client           *http.Client
	enhanced         *EnhancedSiteChecker
	onUpdateCallback func([]*SiteInfo) // Callback for notifying updates
}

// NewSiteChecker creates a new site checker that reuses the package-level
// shared HTTP transport. The shared transport bounds connections per host so
// the checker cannot exhaust router conntrack tables (#1608).
func NewSiteChecker(options CheckOptions) *SiteChecker {
	client := SharedClient(options.Timeout)

	if !options.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if options.MaxRedirects > 0 {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= options.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", options.MaxRedirects)
			}
			return nil
		}
	}

	return &SiteChecker{
		sites:    make(map[string]*SiteInfo),
		options:  options,
		client:   client,
		enhanced: NewEnhancedSiteChecker(),
	}
}

// SetUpdateCallback sets the callback function for site updates
func (sc *SiteChecker) SetUpdateCallback(callback func([]*SiteInfo)) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.onUpdateCallback = callback
}

// CollectSites collects URLs from enabled indexed sites only
func (sc *SiteChecker) CollectSites() {
	reconcileSiteConfigSiteIDsOnce()
	reconcileSiteConfigSiteIDsIfDirty()

	sc.mu.Lock()
	defer sc.mu.Unlock()

	previousSites := sc.sites
	// Clear existing sites
	sc.sites = make(map[string]*SiteInfo)

	// Get a thread-safe snapshot of indexed sites to avoid concurrent map access
	indexedSites := site.GetAllIndexedSites()

	// Debug: log indexed sites count
	logger.Debugf("Found %d indexed sites", len(indexedSites))

	// Collect URLs from indexed sites, but only from enabled sites
	for siteName, indexedSite := range indexedSites {
		// Check site status - only collect from enabled sites
		siteStatus := site.GetSiteStatus(siteName)
		if siteStatus != site.StatusEnabled {
			logger.Debugf("Skipping site %s (status: %s) - only collecting from enabled sites", siteName, siteStatus)
			continue
		}

		logger.Debugf("Processing enabled site: %s with %d URLs", siteName, len(indexedSite.Urls))
		for _, url := range indexedSite.Urls {
			if url != "" {
				logger.Debugf("Adding site URL: %s", url)
				config := getOrCreateSiteConfigForURL(siteName, url)
				protocol := "http" // default protocol
				if config.HealthCheckConfig != nil && config.HealthCheckConfig.Protocol != "" {
					protocol = config.HealthCheckConfig.Protocol
					logger.Debugf("Site %s using protocol: %s", url, protocol)
				} else {
					logger.Debugf("Site %s using default protocol: %s", url, protocol)
				}

				siteInfo := &SiteInfo{
					SiteConfig:                  *config,
					Name:                        extractDomainName(url),
					Status:                      StatusChecking,
					EffectiveHealthCheckEnabled: settings.SiteCheckSettings.Enabled && config.HealthCheckEnabled,
				}
				if previous := previousSites[url]; previous != nil {
					siteInfo.Status = previous.Status
					siteInfo.StatusCode = previous.StatusCode
					siteInfo.ResponseTime = previous.ResponseTime
					siteInfo.FaviconURL = previous.FaviconURL
					siteInfo.FaviconData = previous.FaviconData
					siteInfo.Title = previous.Title
					siteInfo.LastChecked = previous.LastChecked
					siteInfo.Error = previous.Error
					siteInfo.ErrorType = previous.ErrorType
					siteInfo.CertDaysRemaining = previous.CertDaysRemaining
				}
				if !settings.SiteCheckSettings.Enabled {
					siteInfo.HealthCheckDisabledReason = "global"
				} else if !config.HealthCheckEnabled {
					siteInfo.HealthCheckDisabledReason = "site"
				}
				sc.sites[url] = siteInfo
			}
		}
	}

	logger.Debugf("Collected %d enabled sites for checking", len(sc.sites))
}

// loadAllSiteConfigs loads all site configs from database and caches them
func loadAllSiteConfigs() error {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	// Skip database operation if query.SiteConfig is nil (e.g., in tests)
	if query.SiteConfig == nil {
		logger.Debugf("Skipping site config batch load - query.SiteConfig is nil (likely in test environment)")
		lastBatchLoad = time.Now()
		return nil
	}

	sc := query.SiteConfig
	configs, err := sc.Find()
	if err != nil {
		return fmt.Errorf("failed to load site configs: %w", err)
	}

	now := time.Now()
	expiry := now.Add(cacheExpiry)

	// Clear existing cache
	siteConfigCache = make(map[string]*siteConfigCacheEntry)

	// Cache all configs
	for _, config := range configs {
		key := config.SiteKey
		if key == "" {
			key = "legacy:" + config.Host
		}
		siteConfigCache[key] = &siteConfigCacheEntry{
			config:    config,
			expiresAt: expiry,
		}
	}

	lastBatchLoad = now
	logger.Debugf("Loaded %d site configs into cache", len(configs))
	return nil
}

// getCachedSiteConfig gets a site config from cache, loading all configs if needed
func getCachedSiteConfig(host string) (*model.SiteConfig, bool) {
	siteConfigMutex.RLock()

	// Check if we need to refresh the cache
	needsRefresh := time.Since(lastBatchLoad) > cacheExpiry

	if needsRefresh {
		siteConfigMutex.RUnlock()
		// Reload all configs if cache is expired
		if err := loadAllSiteConfigs(); err != nil {
			logger.Errorf("Failed to reload site configs: %v", err)
			return nil, false
		}
		siteConfigMutex.RLock()
	}

	entry, exists := siteConfigCache[host]
	siteConfigMutex.RUnlock()

	if !exists || time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.config, true
}

// setCachedSiteConfig sets a site config in cache
func setCachedSiteConfig(host string, config *model.SiteConfig) {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	siteConfigCache[host] = &siteConfigCacheEntry{
		config:    config,
		expiresAt: time.Now().Add(cacheExpiry),
	}
}

// InvalidateSiteConfigCache invalidates the entire site config cache
func InvalidateSiteConfigCache() {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	siteConfigCache = make(map[string]*siteConfigCacheEntry)
	lastBatchLoad = time.Time{} // Reset batch load time to force reload
	logger.Debugf("Site config cache invalidated")
}

// InvalidateSiteConfigCacheForHost invalidates cache for a specific host
func InvalidateSiteConfigCacheForHost(host string) {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	delete(siteConfigCache, host)
	logger.Debugf("Site config cache invalidated for host: %s", host)
}

func canonicalSiteKeyWithIndex(siteIndex uint64, siteName, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		prefix := strings.TrimSpace(siteName)
		if siteIndex > 0 {
			prefix = fmt.Sprintf("#%d", siteIndex)
		}
		return prefix + "|" + strings.TrimSpace(rawURL)
	}

	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port == "" {
		switch strings.ToLower(parsed.Scheme) {
		case "https", "grpcs":
			port = "443"
		default:
			port = "80"
		}
	}

	prefix := strings.TrimSpace(siteName)
	if siteIndex > 0 {
		prefix = fmt.Sprintf("#%d", siteIndex)
	}

	return prefix + "|" + strings.ToLower(parsed.Scheme) + "://" + net.JoinHostPort(host, port)
}

func canonicalSiteKey(siteName, rawURL string) string {
	return canonicalSiteKeyWithIndex(0, siteName, rawURL)
}

func resolveSiteIndexByName(siteName string) uint64 {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return 0
	}

	path, err := site.ResolveAvailablePath(siteName)
	if err != nil {
		return 0
	}

	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		logger.Warnf("Failed to resolve site index for %s: %v", siteName, err)
		return 0
	}

	return siteModel.ID
}

func countSiteConfigsWithEmptyIndex() int64 {
	var count int64
	err := model.UseDB().Model(&model.SiteConfig{}).
		Where("site_index IS NULL OR site_index = 0").
		Count(&count).Error
	if err != nil {
		logger.Errorf("Failed to count site configs with empty index: %v", err)
		return 0
	}
	return count
}

func countDuplicatedSiteIndexes() int64 {
	var count int64
	err := model.UseDB().Model(&model.SiteConfig{}).
		Select("site_index").
		Where("site_index > 0").
		Group("site_index").
		Having("COUNT(*) > 1").
		Count(&count).Error
	if err != nil {
		logger.Errorf("Failed to count duplicated site_index groups: %v", err)
		return 0
	}
	return count
}

func duplicatedSiteIndexes(db *gorm.DB) ([]uint64, error) {
	type duplicateIndexRow struct {
		SiteIndex uint64
	}

	var rows []duplicateIndexRow
	err := db.Model(&model.SiteConfig{}).
		Select("site_index").
		Where("site_index > 0").
		Group("site_index").
		Having("COUNT(*) > 1").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	indexes := make([]uint64, 0, len(rows))
	for _, row := range rows {
		indexes = append(indexes, row.SiteIndex)
	}

	return indexes, nil
}

func markSiteConfigsDeletedByIndexes(db *gorm.DB, indexes []uint64) (int64, error) {
	if len(indexes) == 0 {
		return 0, nil
	}

	result := db.Where("site_index IN ?", indexes).Delete(&model.SiteConfig{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func resetAllSiteCheckTables(reason string) error {
	db := model.UseDB()
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SiteConfig{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SiteHealthAlertState{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	InvalidateSiteConfigCache()
	logger.Warnf("Sitecheck tables reset: reason=%s", reason)
	return nil
}

func buildIndexedSiteHostLookup() map[string]string {
	lookup := make(map[string]string)
	indexedSites := site.GetAllIndexedSites()

	for siteName, indexedSite := range indexedSites {
		for _, rawURL := range indexedSite.Urls {
			if strings.TrimSpace(rawURL) == "" {
				continue
			}

			tmp := &model.SiteConfig{}
			if err := tmp.SetFromURL(rawURL); err != nil {
				continue
			}
			if tmp.Host == "" {
				continue
			}

			if _, exists := lookup[tmp.Host]; !exists {
				lookup[tmp.Host] = siteName
			}
		}
	}

	return lookup
}

func deduplicateSiteConfigs(tx *gorm.DB) (int, error) {
	type duplicateIndexHost struct {
		SiteIndex uint64
		Host      string
		Scheme    string
	}
	type duplicateSiteKey struct {
		SiteKey string
	}

	idsToDelete := make(map[uint64]struct{})

	markStaleRows := func(where string, args ...any) error {
		var rows []model.SiteConfig
		if err := tx.Where(where, args...).Order("updated_at desc, id desc").Find(&rows).Error; err != nil {
			return err
		}
		for i := 1; i < len(rows); i++ {
			idsToDelete[rows[i].ID] = struct{}{}
		}
		return nil
	}

	var indexHostGroups []duplicateIndexHost
	if err := tx.Model(&model.SiteConfig{}).
		Select("site_index, host, scheme").
		Where("site_index > 0 AND host <> ''").
		Group("site_index, host, scheme").
		Having("COUNT(*) > 1").
		Scan(&indexHostGroups).Error; err != nil {
		return 0, err
	}

	for _, group := range indexHostGroups {
		if err := markStaleRows("site_index = ? AND host = ? AND scheme = ?", group.SiteIndex, group.Host, group.Scheme); err != nil {
			return 0, err
		}
	}

	var siteKeyGroups []duplicateSiteKey
	if err := tx.Model(&model.SiteConfig{}).
		Select("site_key").
		Where("site_key <> ''").
		Group("site_key").
		Having("COUNT(*) > 1").
		Scan(&siteKeyGroups).Error; err != nil {
		return 0, err
	}

	for _, group := range siteKeyGroups {
		if err := markStaleRows("site_key = ?", group.SiteKey); err != nil {
			return 0, err
		}
	}

	if len(idsToDelete) == 0 {
		return 0, nil
	}

	ids := make([]uint64, 0, len(idsToDelete))
	for id := range idsToDelete {
		ids = append(ids, id)
	}

	if err := tx.Where("id IN ?", ids).Delete(&model.SiteConfig{}).Error; err != nil {
		return 0, err
	}

	return len(ids), nil
}

// updateSiteListFieldsOnly persists only sitelist-linked columns for an
// existing site config, so health-check configuration columns are never
// overwritten during reconciliation.
func updateSiteListFieldsOnly(db *gorm.DB, cfg *model.SiteConfig, fields map[string]any) error {
	if cfg == nil || cfg.ID == 0 || len(fields) == 0 {
		return nil
	}

	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	sort.Strings(fieldNames)
	logger.Debugf("Site config sitelist-field update: id=%d fields=%s", cfg.ID, strings.Join(fieldNames, ","))

	if err := db.Model(&model.SiteConfig{}).Where("id = ?", cfg.ID).Updates(fields).Error; err != nil {
		return err
	}

	if v, ok := fields["site_key"].(string); ok {
		cfg.SiteKey = v
	}
	if v, ok := fields["site_name"].(string); ok {
		cfg.SiteName = v
	}
	if v, ok := fields["site_id"].(uint64); ok {
		cfg.SiteID = v
	}
	if v, ok := fields["site_index"].(uint64); ok {
		cfg.SiteIndex = v
	}
	if v, ok := fields["host"].(string); ok {
		cfg.Host = v
	}
	if v, ok := fields["scheme"].(string); ok {
		cfg.Scheme = v
	}
	if v, ok := fields["display_url"].(string); ok {
		cfg.DisplayURL = v
	}
	if v, ok := fields["port"].(int); ok {
		cfg.Port = v
	}

	return nil
}

func upgradeSiteConfigAssociations() (updated int, deduplicated int, unresolved int, err error) {
	tx := model.UseDB().Begin()
	if tx.Error != nil {
		return 0, 0, 0, tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var configs []model.SiteConfig
	if err = tx.Find(&configs).Error; err != nil {
		tx.Rollback()
		return 0, 0, 0, err
	}

	hostLookup := buildIndexedSiteHostLookup()

	for i := range configs {
		cfg := &configs[i]
		fieldsToUpdate := map[string]any{}

		siteName := strings.TrimSpace(cfg.SiteName)
		if siteName == "" {
			if inferred, ok := hostLookup[cfg.Host]; ok {
				siteName = inferred
				fieldsToUpdate["site_name"] = inferred
			}
		}

		targetIndex := cfg.SiteIndex
		if siteName != "" {
			if resolved := resolveSiteIndexByName(siteName); resolved > 0 {
				targetIndex = resolved
			}
		}

		if targetIndex > 0 {
			if cfg.SiteIndex != targetIndex {
				fieldsToUpdate["site_index"] = targetIndex
			}
			if cfg.SiteID != targetIndex {
				fieldsToUpdate["site_id"] = targetIndex
			}
		} else {
			unresolved++
		}

		displayURL := strings.TrimSpace(cfg.DisplayURL)
		if displayURL == "" {
			displayURL = cfg.GetURL()
		}
		expectedSiteKey := canonicalSiteKeyWithIndex(targetIndex, siteName, displayURL)
		if cfg.SiteKey != expectedSiteKey {
			fieldsToUpdate["site_key"] = expectedSiteKey
		}

		if len(fieldsToUpdate) > 0 {
			if err = updateSiteListFieldsOnly(tx, cfg, fieldsToUpdate); err != nil {
				tx.Rollback()
				return 0, 0, 0, err
			}
			updated++
		}
	}

	deduplicated, err = deduplicateSiteConfigs(tx)
	if err != nil {
		tx.Rollback()
		return 0, 0, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return 0, 0, 0, err
	}

	return updated, deduplicated, unresolved, nil
}

func reconcileSiteConfigSiteIDsIfDirty() {
	if query.SiteConfig == nil || query.Site == nil {
		return
	}

	emptyCount := countSiteConfigsWithEmptyIndex()
	duplicatedGroups := countDuplicatedSiteIndexes()
	if emptyCount == 0 && duplicatedGroups == 0 {
		return
	}

	if duplicatedGroups > 0 {
		indexes, idxErr := duplicatedSiteIndexes(model.UseDB())
		if idxErr != nil {
			logger.Errorf("Failed to query duplicated site_index values before reset: %v", idxErr)
		} else {
			deleted, delErr := markSiteConfigsDeletedByIndexes(model.UseDB(), indexes)
			if delErr != nil {
				logger.Errorf("Failed to batch mark duplicated site_index rows deleted before reset: %v", delErr)
			} else if deleted > 0 {
				logger.Warnf("Batch marked duplicated site_index rows deleted before reset: groups=%d rows=%d", duplicatedGroups, deleted)
			}
		}

		reason := fmt.Sprintf("duplicate_site_index_groups=%d empty_index_rows=%d", duplicatedGroups, emptyCount)
		if err := resetAllSiteCheckTables(reason); err != nil {
			logger.Errorf("Failed to reset sitecheck tables for duplicated site_index groups: %v", err)
		}
		return
	}

	updated, deduplicated, unresolved, err := upgradeSiteConfigAssociations()
	if err != nil {
		reason := fmt.Sprintf("upgrade_failed empty_index_rows=%d error=%v", emptyCount, err)
		logger.Warnf("Site config auto-upgrade failed, fallback reset starts: %s", reason)
		if resetErr := resetAllSiteCheckTables(reason); resetErr != nil {
			logger.Errorf("Failed to reset sitecheck tables after upgrade failure: %v", resetErr)
		}
		return
	}

	if updated > 0 || deduplicated > 0 {
		InvalidateSiteConfigCache()
	}

	if unresolved > 0 {
		logger.Warnf("Site config auto-upgrade finished with unresolved records: unresolved=%d", unresolved)
	}

	remainingEmpty := countSiteConfigsWithEmptyIndex()
	if remainingEmpty > 0 {
		logger.Warnf("Site config auto-upgrade could not resolve all empty indexes: remaining=%d", remainingEmpty)
	}

	if updated > 0 || deduplicated > 0 || unresolved > 0 {
		logger.Infof("Site config auto-upgrade summary: updated=%d deduplicated=%d unresolved=%d", updated, deduplicated, unresolved)
	}
}

// ReconcileSiteConfigSiteIDsIfDirty runs the reconciliation flow when site
// config records are dirty (empty or duplicated indexes). This is used by
// write APIs to trigger cleanup immediately instead of waiting for periodic
// site collection.
func ReconcileSiteConfigSiteIDsIfDirty() {
	reconcileSiteConfigSiteIDsIfDirty()
}

func reconcileSiteConfigSiteIDsOnce() {
	linkReconcileOnce.Do(reconcileSiteConfigSiteIDsIfDirty)
}

func splitHostAndPort(hostport string) (string, string) {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return "", ""
	}
	h, p, err := net.SplitHostPort(hostport)
	if err != nil {
		return strings.ToLower(hostport), ""
	}
	return strings.ToLower(h), p
}

func findSiteConfigByIndex(siteIndex uint64, siteName, host, scheme string) (*model.SiteConfig, error) {
	if siteIndex == 0 {
		return nil, fmt.Errorf("invalid site index")
	}

	var duplicateCount int64
	if err := model.UseDB().Model(&model.SiteConfig{}).Where("site_index = ?", siteIndex).Count(&duplicateCount).Error; err != nil {
		return nil, err
	}
	if duplicateCount > 1 {
		deleted, delErr := markSiteConfigsDeletedByIndexes(model.UseDB(), []uint64{siteIndex})
		if delErr != nil {
			logger.Errorf("Failed to batch mark duplicated site_index rows deleted during lookup: index=%d err=%v", siteIndex, delErr)
		} else if deleted > 0 {
			logger.Warnf("Batch marked duplicated site_index rows deleted during lookup: index=%d rows=%d", siteIndex, deleted)
		}

		if err := resetAllSiteCheckTables(fmt.Sprintf("duplicate site_index detected during lookup: %d", siteIndex)); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("site check tables reset due to duplicate site index")
	}

	var candidates []model.SiteConfig
	q := model.UseDB().Where("site_index = ?", siteIndex)
	if strings.TrimSpace(siteName) != "" {
		q = q.Where("site_name = ?", siteName)
	}
	err := q.Order("updated_at desc, id desc").Find(&candidates).Error
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 && strings.TrimSpace(siteName) != "" {
		err = model.UseDB().Where("site_index = ?", siteIndex).Order("updated_at desc, id desc").Find(&candidates).Error
		if err != nil {
			return nil, err
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("site config not found by index")
	}

	targetHost, _ := splitHostAndPort(host)
	targetScheme := strings.ToLower(strings.TrimSpace(scheme))

	for i := range candidates {
		if strings.EqualFold(candidates[i].Host, host) {
			return &candidates[i], nil
		}
	}

	for i := range candidates {
		candidateHost, _ := splitHostAndPort(candidates[i].Host)
		if candidateHost == targetHost {
			if targetScheme == "" || strings.EqualFold(candidates[i].Scheme, targetScheme) {
				return &candidates[i], nil
			}
		}
	}

	return &candidates[0], nil
}

// getOrCreateSiteConfigForURL gets or creates a site config for the given URL.
// SiteKey is stable across synchronized nodes and avoids relying on local IDs.
func getOrCreateSiteConfigForURL(siteName, rawURL string) *model.SiteConfig {
	// Parse URL to get host:port
	tempConfig := &model.SiteConfig{}
	tempConfig.SetFromURL(rawURL)
	tempConfig.SiteName = siteName
	tempConfig.SiteIndex = resolveSiteIndexByName(siteName)
	tempConfig.SiteID = tempConfig.SiteIndex
	tempConfig.SiteKey = canonicalSiteKeyWithIndex(tempConfig.SiteIndex, siteName, rawURL)
	legacySiteKey := canonicalSiteKey(siteName, rawURL)

	// Try to get from cache first
	if config, found := getCachedSiteConfig(tempConfig.SiteKey); found {
		fieldsToUpdate := map[string]any{}
		if tempConfig.SiteIndex > 0 && config.SiteIndex != tempConfig.SiteIndex {
			fieldsToUpdate["site_index"] = tempConfig.SiteIndex
		}
		if tempConfig.SiteID > 0 && config.SiteID != tempConfig.SiteID {
			fieldsToUpdate["site_id"] = tempConfig.SiteID
		}
		if strings.TrimSpace(siteName) != "" && strings.TrimSpace(config.SiteName) != strings.TrimSpace(siteName) {
			fieldsToUpdate["site_name"] = siteName
		}
		if len(fieldsToUpdate) > 0 {
			_ = updateSiteListFieldsOnly(model.UseDB(), config, fieldsToUpdate)
		}
		return config
	}
	if config, found := getCachedSiteConfig(legacySiteKey); found {
		fieldsToUpdate := map[string]any{}
		if config.SiteKey != tempConfig.SiteKey {
			fieldsToUpdate["site_key"] = tempConfig.SiteKey
		}
		if tempConfig.SiteIndex > 0 && config.SiteIndex != tempConfig.SiteIndex {
			fieldsToUpdate["site_index"] = tempConfig.SiteIndex
		}
		if tempConfig.SiteID > 0 && config.SiteID != tempConfig.SiteID {
			fieldsToUpdate["site_id"] = tempConfig.SiteID
		}
		if strings.TrimSpace(siteName) != "" && strings.TrimSpace(config.SiteName) != strings.TrimSpace(siteName) {
			fieldsToUpdate["site_name"] = siteName
		}
		if len(fieldsToUpdate) > 0 {
			_ = updateSiteListFieldsOnly(model.UseDB(), config, fieldsToUpdate)
		}
		setCachedSiteConfig(tempConfig.SiteKey, config)
		return config
	}

	// Not in cache, query database
	sc := query.SiteConfig
	siteConfig, err := sc.Where(sc.SiteKey.Eq(tempConfig.SiteKey)).First()
	if err != nil {
		siteConfig, err = sc.Where(sc.SiteKey.Eq(legacySiteKey)).First()
	}
	if err != nil {
		if tempConfig.SiteIndex > 0 {
			siteConfig, err = findSiteConfigByIndex(tempConfig.SiteIndex, siteName, tempConfig.Host, tempConfig.Scheme)
			if err != nil {
				siteConfig, err = findSiteConfigByIndex(tempConfig.SiteIndex, siteName, "", tempConfig.Scheme)
			}
		}
	}
	if err != nil && strings.TrimSpace(siteName) != "" {
		siteConfig, err = sc.Where(sc.Host.Eq(tempConfig.Host), sc.SiteName.Eq(siteName)).First()
	}
	if err != nil {
		// Lazily adopt a legacy record when it still has no stable key.
		siteConfig, err = sc.Where(sc.Host.Eq(tempConfig.Host), sc.SiteKey.Eq("")).First()
		if err == nil {
			fieldsToUpdate := map[string]any{
				"site_key": tempConfig.SiteKey,
			}
			if strings.TrimSpace(siteName) != "" {
				fieldsToUpdate["site_name"] = siteName
			}
			if tempConfig.SiteIndex > 0 {
				fieldsToUpdate["site_index"] = tempConfig.SiteIndex
			}
			if tempConfig.SiteID > 0 {
				fieldsToUpdate["site_id"] = tempConfig.SiteID
			}
			if saveErr := updateSiteListFieldsOnly(model.UseDB(), siteConfig, fieldsToUpdate); saveErr != nil {
				logger.Errorf("Failed to migrate legacy site config for %s: %v", rawURL, saveErr)
			}
			setCachedSiteConfig(tempConfig.SiteKey, siteConfig)
			return siteConfig
		}

		// Record doesn't exist, create a new one
		newConfig := &model.SiteConfig{
			SiteKey:            tempConfig.SiteKey,
			SiteName:           siteName,
			SiteID:             tempConfig.SiteID,
			SiteIndex:          tempConfig.SiteIndex,
			Host:               tempConfig.Host,
			Port:               tempConfig.Port,
			Scheme:             tempConfig.Scheme,
			DisplayURL:         rawURL,
			HealthCheckEnabled: true,
			CheckInterval:      300,
			Timeout:            10,
			UserAgent:          "Nginx-UI Site Checker/1.0",
			MaxRedirects:       3,
			FollowRedirects:    true,
			CheckFavicon:       true,
		}

		// Create the record in database
		if err := sc.Create(newConfig); err != nil {
			logger.Errorf("Failed to create site config for %s: %v", rawURL, err)
			// Return temp config with a fake ID to avoid crashes
			tempConfig.ID = 0
			return tempConfig
		}

		// Cache the new config
		setCachedSiteConfig(tempConfig.SiteKey, newConfig)
		return newConfig
	}

	// Record exists, update only sitelist-linked fields.
	fieldsToUpdate := map[string]any{}
	if strings.TrimSpace(siteConfig.DisplayURL) != strings.TrimSpace(rawURL) {
		fieldsToUpdate["display_url"] = rawURL
	}

	if siteConfig.Host != tempConfig.Host || siteConfig.Port != tempConfig.Port || siteConfig.Scheme != tempConfig.Scheme {
		fieldsToUpdate["host"] = tempConfig.Host
		fieldsToUpdate["port"] = tempConfig.Port
		fieldsToUpdate["scheme"] = tempConfig.Scheme
	}

	if siteConfig.SiteKey != tempConfig.SiteKey {
		fieldsToUpdate["site_key"] = tempConfig.SiteKey
	}

	if strings.TrimSpace(siteConfig.SiteName) != strings.TrimSpace(siteName) {
		fieldsToUpdate["site_name"] = siteName
	}

	if tempConfig.SiteIndex > 0 && siteConfig.SiteIndex != tempConfig.SiteIndex {
		fieldsToUpdate["site_index"] = tempConfig.SiteIndex
	}

	if tempConfig.SiteID > 0 && siteConfig.SiteID != tempConfig.SiteID {
		fieldsToUpdate["site_id"] = tempConfig.SiteID
	}

	if len(fieldsToUpdate) > 0 {
		// Try to persist sitelist-linked fields only, but don't fail the request if update fails.
		_ = updateSiteListFieldsOnly(model.UseDB(), siteConfig, fieldsToUpdate)
	}

	// Cache the config
	setCachedSiteConfig(tempConfig.SiteKey, siteConfig)
	return siteConfig
}

// CheckSite checks a single site's availability
func (sc *SiteChecker) CheckSite(ctx context.Context, siteURL string) (*SiteInfo, error) {
	return sc.checkSite(ctx, "", siteURL)
}

// ProbeResult is the outcome of checking one site, without the surrounding
// bookkeeping that SiteInfo carries.
type ProbeResult struct {
	Status       string
	StatusCode   int
	ResponseTime int64
	Title        string
	Error        string
	ErrorType    string
}

// Prober overrides how a site is probed.
//
// The slot defaults to nil and only internal/demo fills it. Returning false
// means "no opinion" and the real HTTP check runs, so an override can never
// answer for a site it does not know about.
type Prober interface {
	Probe(siteURL string) (ProbeResult, bool)
}

var prober Prober

// SetProber installs a probe override. Call once, at boot.
func SetProber(p Prober) {
	prober = p
}

// siteInfoFromProbe assembles a SiteInfo from a fabricated outcome, reusing the
// site's real configuration so only the probe result itself is substituted.
func (sc *SiteChecker) siteInfoFromProbe(siteName, siteURL string, config *model.SiteConfig, outcome ProbeResult) *SiteInfo {
	if config == nil {
		config = getOrCreateSiteConfigForURL(siteName, siteURL)
	}

	title := outcome.Title
	if title == "" {
		title = extractDomainName(siteURL)
	}

	return &SiteInfo{
		SiteConfig:                  *config,
		Name:                        extractDomainName(siteURL),
		Status:                      outcome.Status,
		StatusCode:                  outcome.StatusCode,
		ResponseTime:                outcome.ResponseTime,
		Title:                       title,
		Error:                       outcome.Error,
		ErrorType:                   outcome.ErrorType,
		LastChecked:                 time.Now().Unix(),
		EffectiveHealthCheckEnabled: settings.SiteCheckSettings.Enabled && config.HealthCheckEnabled,
	}
}

func (sc *SiteChecker) checkSite(ctx context.Context, siteName, siteURL string) (*SiteInfo, error) {
	// Try enhanced health check first if config exists
	config, err := LoadSiteConfig(siteName, siteURL)

	// If health check is disabled, return cached metadata without issuing any network requests (#1446)
	if err == nil && config != nil && !config.HealthCheckEnabled {
		siteInfo := &SiteInfo{
			SiteConfig:                  *config,
			Name:                        extractDomainName(siteURL),
			Title:                       config.DisplayURL,
			EffectiveHealthCheckEnabled: false,
			HealthCheckDisabledReason:   "site",
		}

		if existing := sc.getExistingSiteSnapshot(siteURL); existing != nil {
			siteInfo.FaviconURL = existing.FaviconURL
			siteInfo.FaviconData = existing.FaviconData
			siteInfo.Status = existing.Status
			siteInfo.StatusCode = existing.StatusCode
			siteInfo.ResponseTime = existing.ResponseTime
			siteInfo.LastChecked = existing.LastChecked
			siteInfo.Error = existing.Error
			siteInfo.ErrorType = existing.ErrorType
			siteInfo.CertDaysRemaining = existing.CertDaysRemaining
			if siteInfo.Title == "" {
				siteInfo.Title = existing.Title
			}
		}

		return siteInfo, nil
	}

	// Substitute the probe result before any network work happens. Placed after
	// the health-check-disabled branch so an operator's "do not check this"
	// still wins over a fabricated result.
	if p := prober; p != nil {
		if outcome, ok := p.Probe(siteURL); ok {
			siteInfo := sc.siteInfoFromProbe(siteName, siteURL, config, outcome)
			if config != nil {
				evaluateSiteHealthAlert(config, siteInfo)
			}
			return siteInfo, nil
		}
	}

	if err == nil && config != nil && config.HealthCheckConfig != nil {
		result, _ := sc.enhanced.CheckSiteWithSiteConfig(ctx, siteURL, config)
		if result != nil && result.Info != nil {
			siteInfo := result.Info
			// Fill in additional details
			siteInfo.SiteConfig = *config
			siteInfo.Name = extractDomainName(siteURL)
			siteInfo.LastChecked = time.Now().Unix()
			siteInfo.EffectiveHealthCheckEnabled = settings.SiteCheckSettings.Enabled && config.HealthCheckEnabled

			// Set health check protocol and display URL
			siteInfo.DisplayURL = generateDisplayURL(siteURL, config.HealthCheckConfig.Protocol)

			// Try to get favicon if enabled and not a gRPC check.
			// Reuse the body fetched by the health check whenever possible
			// to avoid issuing a second GET to the same host (#1608).
			if config.CheckFavicon && !isGRPCProtocol(config.HealthCheckConfig.Protocol) {
				faviconURL, faviconData := sc.faviconFromBody(ctx, effectiveHealthCheckURL(siteURL, config.HealthCheckConfig), result.Body)
				siteInfo.FaviconURL = faviconURL
				siteInfo.FaviconData = faviconData
			}

			evaluateSiteHealthAlert(config, siteInfo)

			return siteInfo, nil
		}
	}

	// Fallback to basic HTTP check, but preserve original protocol if available
	originalProtocol := "http" // default
	if config != nil && config.HealthCheckConfig != nil && config.HealthCheckConfig.Protocol != "" {
		originalProtocol = config.HealthCheckConfig.Protocol
	}
	siteInfo, err := sc.checkSiteBasic(ctx, siteName, siteURL, originalProtocol)
	if siteInfo != nil && config != nil {
		siteInfo.EffectiveHealthCheckEnabled = settings.SiteCheckSettings.Enabled && config.HealthCheckEnabled
		evaluateSiteHealthAlert(config, siteInfo)
	}
	return siteInfo, err
}

// checkSiteBasic performs basic HTTP health check
func (sc *SiteChecker) checkSiteBasic(ctx context.Context, siteName, siteURL string, originalProtocol string) (*SiteInfo, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", siteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", sc.options.UserAgent)

	resp, err := sc.client.Do(req)
	if err != nil {
		// Get or create site config to get ID
		siteConfig := getOrCreateSiteConfigForURL(siteName, siteURL)

		return &SiteInfo{
			SiteConfig:   *siteConfig,
			Name:         extractDomainName(siteURL),
			Status:       StatusOffline,
			ResponseTime: time.Since(start).Milliseconds(),
			LastChecked:  time.Now().Unix(),
			Error:        err.Error(),
			ErrorType:    classifyCheckError(err),
		}, nil
	}
	defer resp.Body.Close()

	responseTime := time.Since(start).Milliseconds()

	// Get or create site config to get ID
	siteConfig := getOrCreateSiteConfigForURL(siteName, siteURL)

	siteInfo := &SiteInfo{
		SiteConfig:        *siteConfig,
		Name:              extractDomainName(siteURL),
		StatusCode:        resp.StatusCode,
		ResponseTime:      responseTime,
		CertDaysRemaining: calculateCertDaysRemaining(resp.TLS),
		LastChecked:       time.Now().Unix(),
	}

	// Determine status based on status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		siteInfo.Status = StatusOnline
	} else {
		siteInfo.Status = StatusError
		siteInfo.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		siteInfo.ErrorType = ErrorTypeStatusCode
	}

	// Read response body for title and favicon extraction
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnf("Failed to read response body for %s: %v", siteURL, err)
		return siteInfo, nil
	}

	// Extract title
	siteInfo.Title = extractTitle(string(body))

	// Extract favicon if enabled
	if sc.options.CheckFavicon {
		faviconURL, faviconData := sc.extractFavicon(ctx, siteURL, string(body))
		siteInfo.FaviconURL = faviconURL
		siteInfo.FaviconData = faviconData
	}

	return siteInfo, nil
}

// faviconFromBody extracts the favicon using a body that has already been
// fetched by the health checker. If the body is empty (e.g. the path probed
// wasn't the homepage), it falls back to a single GET via the shared client.
func (sc *SiteChecker) faviconFromBody(ctx context.Context, siteURL string, body []byte) (string, string) {
	if len(body) > 0 {
		return sc.extractFavicon(ctx, siteURL, string(body))
	}
	return sc.tryGetFavicon(ctx, siteURL)
}

// tryGetFavicon attempts to get favicon when no health-check body is
// available (e.g. gRPC checks, or when the enhanced check failed and we are
// falling back). It uses the shared transport so it shares the per-host
// connection budget with the health check itself.
func (sc *SiteChecker) tryGetFavicon(ctx context.Context, siteURL string) (string, string) {
	req, err := http.NewRequestWithContext(ctx, "GET", siteURL, nil)
	if err != nil {
		return "", ""
	}

	req.Header.Set("User-Agent", sc.options.UserAgent)

	resp, err := sc.client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	return sc.extractFavicon(ctx, siteURL, string(body))
}

// CheckAllSites checks all collected sites concurrently. URLs sharing the
// same host:port are coalesced into a single network probe whose result is
// fanned out to every alias, so multi-server_name configs do not multiply
// outbound connections (#1608).
func (sc *SiteChecker) CheckAllSites(ctx context.Context) {
	sc.checkAllSites(ctx, false)
}

func (sc *SiteChecker) ForceCheckAllSites(ctx context.Context) {
	sc.checkAllSites(ctx, true)
}

func (sc *SiteChecker) checkAllSites(ctx context.Context, force bool) {
	if !settings.SiteCheckSettings.Enabled {
		logger.Debug("Site check is disabled; skipping CheckAllSites")
		sc.broadcastUpdate()
		return
	}

	sc.mu.RLock()
	urls := make([]string, 0, len(sc.sites))
	for url, siteInfo := range sc.sites {
		if !force && siteInfo != nil && siteInfo.CheckInterval > 0 && siteInfo.LastChecked > 0 &&
			time.Since(time.Unix(siteInfo.LastChecked, 0)) < time.Duration(siteInfo.CheckInterval)*time.Second {
			continue
		}
		urls = append(urls, url)
	}
	sc.mu.RUnlock()

	// Group URLs by effective target and request behavior so aliases can share a
	// probe without accidentally applying one site's custom configuration to
	// another site.
	groups := make(map[string][]string, len(urls))
	for _, raw := range urls {
		sc.mu.RLock()
		snapshot := sc.sites[raw]
		sc.mu.RUnlock()
		key := dedupeKey(raw)
		if snapshot != nil {
			target := effectiveHealthCheckURL(raw, snapshot.HealthCheckConfig)
			fingerprint, _ := json.Marshal(struct {
				Config          *model.HealthCheckConfig
				Timeout         int
				UserAgent       string
				MaxRedirects    int
				FollowRedirects bool
			}{
				Config:          snapshot.HealthCheckConfig,
				Timeout:         snapshot.Timeout,
				UserAgent:       snapshot.UserAgent,
				MaxRedirects:    snapshot.MaxRedirects,
				FollowRedirects: snapshot.FollowRedirects,
			})
			key = dedupeKey(target) + "|" + string(fingerprint)
		}
		groups[key] = append(groups[key], raw)
	}

	concurrency := settings.SiteCheckSettings.GetConcurrency()
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, aliases := range groups {
		wg.Add(1)
		go func(aliases []string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			primary := aliases[0]
			sc.mu.RLock()
			primarySnapshot := sc.sites[primary]
			siteName := ""
			if primarySnapshot != nil {
				siteName = primarySnapshot.SiteName
			}
			sc.mu.RUnlock()
			siteInfo, err := sc.checkSite(ctx, siteName, primary)
			if err != nil {
				logger.Errorf("Failed to check site %s: %v", primary, err)
				return
			}

			sc.mu.Lock()
			for _, alias := range aliases {
				if alias == primary {
					sc.sites[alias] = siteInfo
					continue
				}
				clone := *siteInfo
				if existing := sc.sites[alias]; existing != nil {
					clone.SiteConfig = existing.SiteConfig
					clone.Name = existing.Name
				}
				sc.sites[alias] = &clone
				evaluateSiteHealthAlert(&clone.SiteConfig, &clone)
			}
			sc.mu.Unlock()
		}(aliases)
	}

	wg.Wait()

	sc.broadcastUpdate()
}

func (sc *SiteChecker) broadcastUpdate() {
	// Notify WebSocket clients of the update
	sc.mu.RLock()
	updateCallback := sc.onUpdateCallback
	if updateCallback == nil {
		sc.mu.RUnlock()
		return
	}
	sites := make([]*SiteInfo, 0, len(sc.sites))
	for _, site := range sc.sites {
		sites = append(sites, site)
	}
	sc.mu.RUnlock()
	updateCallback(sites)
}

// dedupeKey builds a stable key for an indexed site URL so aliases that
// resolve to the same host:port share a single network probe.
func dedupeKey(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	host := strings.ToLower(parsed.Host)
	if !strings.Contains(host, ":") {
		switch parsed.Scheme {
		case "https", "grpcs":
			host += ":443"
		default:
			host += ":80"
		}
	}
	return parsed.Scheme + "://" + host
}

// GetSites returns all checked sites
func (sc *SiteChecker) GetSites() map[string]*SiteInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*SiteInfo)
	maps.Copy(result, sc.sites)
	return result
}

// GetSiteCount returns the number of sites being monitored
func (sc *SiteChecker) GetSiteCount() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.sites)
}

// GetSitesList returns sites as a slice
func (sc *SiteChecker) GetSitesList() []*SiteInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make([]*SiteInfo, 0, len(sc.sites))
	for _, site := range sc.sites {
		result = append(result, site)
	}
	return result
}

// getExistingSiteSnapshot returns a copy of the last known site info, if present.
func (sc *SiteChecker) getExistingSiteSnapshot(siteURL string) *SiteInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	existing, ok := sc.sites[siteURL]
	if !ok || existing == nil {
		return nil
	}

	clone := *existing
	return &clone
}

// extractDomainName extracts domain name from URL
func extractDomainName(siteURL string) string {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return siteURL
	}
	return parsed.Host
}

// HTML extraction patterns. Compiled once: they are applied to every site on
// every check sweep, and regexp.MustCompile is expensive enough to show up when
// a forced sweep walks hundreds of sites.
var (
	titleRegex   = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	faviconRegex = regexp.MustCompile(`(?i)<link[^>]*rel=["'](?:icon|shortcut icon)["'][^>]*href=["']([^"']+)["']`)
)

// extractTitle extracts title from HTML content
func extractTitle(html string) string {
	matches := titleRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractFavicon extracts favicon URL and data from HTML
func (sc *SiteChecker) extractFavicon(ctx context.Context, siteURL, html string) (string, string) {
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		return "", ""
	}

	// Look for favicon link in HTML
	matches := faviconRegex.FindStringSubmatch(html)

	var faviconURL string
	if len(matches) > 1 {
		faviconURL = matches[1]
	} else {
		// Default favicon location
		faviconURL = "/favicon.ico"
	}

	// Convert relative URL to absolute
	if !strings.HasPrefix(faviconURL, "http") {
		baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
		if strings.HasPrefix(faviconURL, "/") {
			faviconURL = baseURL + faviconURL
		} else {
			faviconURL = baseURL + "/" + faviconURL
		}
	}

	// Download favicon
	faviconData := sc.downloadFavicon(ctx, faviconURL)

	return faviconURL, faviconData
}

// downloadFavicon downloads and encodes favicon as base64
// maxFaviconBytes bounds how much of a favicon response is kept.
//
// The result is base64-encoded (a third larger again), stored in SiteInfo for
// the life of the process, cloned per alias, and re-marshalled to every
// WebSocket client on each update. Real favicons are a few KB; the previous 1MB
// allowance let a few hundred sites pin hundreds of megabytes of resident
// memory for no benefit.
const maxFaviconBytes = 64 * 1024

func (sc *SiteChecker) downloadFavicon(ctx context.Context, faviconURL string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", faviconURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", sc.options.UserAgent)

	resp, err := sc.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFaviconBytes))
	if err != nil {
		return ""
	}

	headerContentType := normalizeContentType(resp.Header.Get("Content-Type"))
	inferredContentType := inferContentTypeFromURL(faviconURL)
	sniffedContentType := normalizeContentType(http.DetectContentType(body))

	contentType := headerContentType
	if !isAllowedFaviconContentType(contentType) && isAllowedFaviconContentType(sniffedContentType) {
		contentType = sniffedContentType
	}
	if !isAllowedFaviconContentType(contentType) &&
		headerContentType == "" &&
		isUnknownContentType(sniffedContentType) &&
		isAllowedFaviconContentType(inferredContentType) {
		contentType = inferredContentType
	}
	if !isAllowedFaviconContentType(contentType) {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(body)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

func normalizeContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	if semi := strings.Index(contentType, ";"); semi != -1 {
		contentType = contentType[:semi]
	}
	return strings.TrimSpace(strings.ToLower(contentType))
}

func inferContentTypeFromURL(faviconURL string) string {
	lower := strings.ToLower(faviconURL)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(lower, ".ico"):
		return "image/x-icon"
	default:
		return ""
	}
}

func isAllowedFaviconContentType(contentType string) bool {
	switch contentType {
	case "image/png",
		"image/jpeg",
		"image/webp",
		"image/gif",
		"image/svg+xml",
		"image/x-icon",
		"image/vnd.microsoft.icon":
		return true
	default:
		return false
	}
}

func isUnknownContentType(contentType string) bool {
	return contentType == "" || contentType == "application/octet-stream"
}

// generateDisplayURL generates the URL to display in UI based on health check protocol
func generateDisplayURL(originalURL, protocol string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		logger.Errorf("Failed to parse URL %s: %v", originalURL, err)
		return originalURL
	}

	// Determine the optimal scheme (prefer HTTPS if available)
	scheme := determineOptimalScheme(parsed, protocol)
	hostname := parsed.Hostname()
	port := parsed.Port()

	// For HTTP/HTTPS, return clean URL without default ports
	if scheme == "http" || scheme == "https" {
		// Build URL without default ports
		var result string
		if port == "" || (port == "80" && scheme == "http") || (port == "443" && scheme == "https") {
			// No port or default port - don't show port
			result = fmt.Sprintf("%s://%s", scheme, hostname)
		} else {
			// Non-default port - show port
			result = fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		}
		return result
	}

	// For gRPC/gRPCS, show the connection address format without default ports
	if scheme == "grpc" || scheme == "grpcs" {
		if port == "" {
			// Determine default port based on scheme
			if scheme == "grpcs" {
				port = "443"
			} else {
				port = "80"
			}
		}

		// Don't show default ports for gRPC either
		var result string
		if (port == "80" && scheme == "grpc") || (port == "443" && scheme == "grpcs") {
			result = fmt.Sprintf("%s://%s", scheme, hostname)
		} else {
			result = fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		}
		return result
	}

	// Fallback to original URL
	return originalURL
}

// isGRPCProtocol checks if the protocol is gRPC-based
func isGRPCProtocol(protocol string) bool {
	return protocol == "grpc" || protocol == "grpcs"
}

// parseURLComponents extracts scheme and host:port from URL based on health check protocol
func parseURLComponents(originalURL, healthCheckProtocol string) (scheme, hostPort string) {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		logger.Debugf("Failed to parse URL %s: %v", originalURL, err)
		return healthCheckProtocol, originalURL
	}

	// Determine the best scheme to use
	scheme = determineOptimalScheme(parsed, healthCheckProtocol)

	// Extract hostname and port
	hostname := parsed.Hostname()
	if hostname == "" {
		// Fallback to original URL if we can't parse hostname
		return scheme, originalURL
	}

	port := parsed.Port()
	if port == "" {
		// Use default port based on scheme, but don't include it in hostPort for default ports
		switch scheme {
		case "https", "grpcs":
			// Default HTTPS port 443 - don't show in hostPort
			hostPort = hostname
		case "http", "grpc":
			// Default HTTP port 80 - don't show in hostPort
			hostPort = hostname
		default:
			hostPort = hostname
		}
	} else {
		// Non-default port specified
		isDefaultPort := (port == "80" && (scheme == "http" || scheme == "grpc")) ||
			(port == "443" && (scheme == "https" || scheme == "grpcs"))

		if isDefaultPort {
			// Don't show default ports
			hostPort = hostname
		} else {
			// Show non-default ports
			hostPort = hostname + ":" + port
		}
	}

	return scheme, hostPort
}

// determineOptimalScheme determines the best scheme to use based on original URL and health check protocol
func determineOptimalScheme(parsed *url.URL, healthCheckProtocol string) string {
	// If health check protocol is specified, use it, but with special handling for HTTP/HTTPS
	if healthCheckProtocol != "" {
		// Special case: Don't downgrade HTTPS to HTTP
		if healthCheckProtocol == "http" && parsed.Scheme == "https" {
			// logger.Debugf("Preserving HTTPS scheme instead of downgrading to HTTP")
			return "https"
		}

		// For gRPC protocols, always use the specified protocol
		if healthCheckProtocol == "grpc" || healthCheckProtocol == "grpcs" {
			return healthCheckProtocol
		}

		// For HTTPS health check protocol, always use HTTPS
		if healthCheckProtocol == "https" {
			return "https"
		}

		// For HTTP health check protocol, only use HTTP if original was also HTTP
		if healthCheckProtocol == "http" && parsed.Scheme == "http" {
			return "http"
		}
	}

	// If no health check protocol, or if we need to fall back, prefer HTTPS if the original URL is HTTPS
	if parsed.Scheme == "https" {
		return "https"
	}

	// Default to HTTP
	return "http"
}
