package sites

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	internalsite "github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetSiteNavigation returns all sites for navigation dashboard
func GetSiteNavigation(c *gin.Context) {
	service := sitecheck.GetService()
	sites := redactSiteInfos(service.GetSites(), healthCheckSecretViewFor(c))

	c.JSON(http.StatusOK, gin.H{
		"data": sites,
	})
}

// GetSiteNavigationStatus returns the status of site checking service
func GetSiteNavigationStatus(c *gin.Context) {
	service := sitecheck.GetService()

	c.JSON(http.StatusOK, gin.H{
		"running":          service.IsRunning(),
		"enabled":          settings.SiteCheckSettings.Enabled,
		"concurrency":      settings.SiteCheckSettings.GetConcurrency(),
		"interval_seconds": settings.SiteCheckSettings.IntervalSeconds,
	})
}

// UpdateSiteOrder updates the custom order of sites
func UpdateSiteOrder(c *gin.Context) {
	var req struct {
		OrderedIds []uint64 `json:"ordered_ids" binding:"required"`
	}

	if !cosy.BindAndValid(c, &req) {
		return
	}

	if err := updateSiteOrderBatchByIds(req.OrderedIds); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order updated successfully",
	})
}

// updateSiteOrderBatchByIds updates site order in batch using IDs
// Uses INSERT INTO ... ON DUPLICATE KEY UPDATE for better performance
func updateSiteOrderBatchByIds(orderedIds []uint64) error {
	if len(orderedIds) == 0 {
		return nil
	}

	sc := query.SiteConfig

	records := make([]*model.SiteConfig, 0, len(orderedIds))
	for i, id := range orderedIds {
		records = append(records, &model.SiteConfig{
			Model:       model.Model{ID: id},
			CustomOrder: i,
		})
	}

	return sc.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"custom_order"}),
	}).Create(records...)
}

// GetHealthCheck gets health check configuration for a site
func GetHealthCheck(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	sc := query.SiteConfig
	siteConfig, err := sc.Where(sc.ID.Eq(id)).First()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	ensureHealthCheckConfig(siteConfig)
	healthCheckSecretViewFor(c).redact(siteConfig)

	c.JSON(http.StatusOK, siteConfig)
}

// healthCheckSecretView says which of the probe secrets a site config stores
// a caller may read.
type healthCheckSecretView uint8

const (
	// showHealthCheckSecrets is the view of the web UI's login token and of
	// node-to-node requests, which need every value to round-trip the editor
	// and to synchronize sites.
	showHealthCheckSecrets healthCheckSecretView = iota
	// hideProbeConfigValues is the view of a service token with api:write. The
	// form values of a plugin probe kind are hidden: the plugin defines that
	// form, so any of its values may be a credential. The other probe settings
	// stay readable.
	hideProbeConfigValues
	// hideHealthCheckSecrets is the view of a read-only service token. On top of
	// the plugin form values it hides custom header values, which commonly
	// carry Authorization credentials, the request body and URL credentials.
	hideHealthCheckSecrets
)

func healthCheckSecretViewFor(c *gin.Context) healthCheckSecretView {
	switch {
	case internalmcp.IsReadOnlyServiceTokenRequest(c):
		return hideHealthCheckSecrets
	case internalmcp.IsServiceTokenRequest(c):
		return hideProbeConfigValues
	default:
		return showHealthCheckSecrets
	}
}

// redact replaces the values the view hides with the shared redaction
// placeholder. Header and form keys stay visible. Values are copied so a
// shared config is never modified.
func (view healthCheckSecretView) redact(siteConfig *model.SiteConfig) {
	if view == showHealthCheckSecrets {
		return
	}
	siteConfig.ProbeConfig = internalmcp.RedactValues(siteConfig.ProbeConfig)
	if view != hideHealthCheckSecrets || siteConfig.HealthCheckConfig == nil {
		return
	}
	healthCheckConfig := *siteConfig.HealthCheckConfig
	healthCheckConfig.Headers = internalmcp.RedactValues(healthCheckConfig.Headers)
	if healthCheckConfig.Body != "" {
		healthCheckConfig.Body = settings.RedactedSensitiveValue
	}
	healthCheckConfig.TargetURL = internalmcp.RedactURLCredentials(healthCheckConfig.TargetURL)
	healthCheckConfig.ClickURL = internalmcp.RedactURLCredentials(healthCheckConfig.ClickURL)
	siteConfig.HealthCheckConfig = &healthCheckConfig
}

// redactSiteInfos returns the checker's sites as the view shows them. Hidden
// values are redacted in copies: the checker keeps probing with the originals,
// so they must not be modified.
func redactSiteInfos(sites []*sitecheck.SiteInfo, view healthCheckSecretView) []*sitecheck.SiteInfo {
	if view == showHealthCheckSecrets {
		return sites
	}
	redacted := make([]*sitecheck.SiteInfo, 0, len(sites))
	for _, site := range sites {
		if site == nil {
			redacted = append(redacted, nil)
			continue
		}
		clone := *site
		view.redact(&clone.SiteConfig)
		redacted = append(redacted, &clone)
	}
	return redacted
}

// restoreRedactedProbeConfig lets a caller save back the probe_config a
// redacted read returned without overwriting the stored values with the
// placeholder. A probe_config that is left out, or that is the redacted map
// itself (the stored keys, each value the placeholder), keeps the stored
// values, but only while the probe stays the same: the same plugin kind and the
// same probe target. Otherwise the request could point the stored credentials
// at a target of its own choosing, so it has to send every value itself. The
// placeholder mixed with other values is refused rather than compared with the
// stored values, so the answer never tells whether a guessed value is right.
// A probe_config without the placeholder is used as sent.
func restoreRedactedProbeConfig(stored *model.SiteConfig, kind string, config *model.HealthCheckConfig, submitted map[string]string) (map[string]string, error) {
	if sitecheck.IsBuiltinProbeKind(kind) {
		return submitted, nil
	}
	if submitted != nil && !carriesRedactionPlaceholder(submitted) {
		return submitted, nil
	}
	if submitted == nil {
		if kind != stored.ProbeKind || len(stored.ProbeConfig) == 0 {
			return nil, nil
		}
		if probeTarget(config) != probeTarget(stored.HealthCheckConfig) {
			return nil, fmt.Errorf("probe_config is required when the probe target changes")
		}
		return maps.Clone(stored.ProbeConfig), nil
	}
	for key, value := range submitted {
		if _, ok := stored.ProbeConfig[key]; !ok || value != settings.RedactedSensitiveValue {
			return nil, fmt.Errorf("probe_config may only carry the redaction placeholder as the stored form returned it, every value the placeholder; to change a value, send every value")
		}
	}
	if len(submitted) != len(stored.ProbeConfig) {
		return nil, fmt.Errorf("probe_config carries the redaction placeholder, but its keys differ from the stored ones; send every value")
	}
	if kind != stored.ProbeKind {
		return nil, fmt.Errorf("probe_config carries the redaction placeholder, but probe_kind differs from the stored kind; send every value")
	}
	if probeTarget(config) != probeTarget(stored.HealthCheckConfig) {
		return nil, fmt.Errorf("probe_config carries the redaction placeholder, but the probe target changed; send every value")
	}
	return maps.Clone(stored.ProbeConfig), nil
}

func carriesRedactionPlaceholder(values map[string]string) bool {
	for _, value := range values {
		if value == settings.RedactedSensitiveValue {
			return true
		}
	}
	return false
}

// probeTarget is the URL a plugin probe kind checks when it is not the site's
// own URL, which this request cannot change.
func probeTarget(config *model.HealthCheckConfig) string {
	if config == nil {
		return ""
	}
	return strings.TrimSpace(config.TargetURL)
}

// createDefaultHealthCheckConfig creates default health check configuration
func createDefaultHealthCheckConfig() *model.HealthCheckConfig {
	return &model.HealthCheckConfig{
		Protocol:       "http",
		Method:         "GET",
		Path:           "/",
		ExpectedStatus: []int{200},
		GRPCMethod:     "Check",
	}
}

// ensureHealthCheckConfig ensures health check config is not nil
func ensureHealthCheckConfig(siteConfig *model.SiteConfig) {
	if siteConfig.HealthCheckConfig == nil {
		siteConfig.HealthCheckConfig = createDefaultHealthCheckConfig()
	}
}

type updateHealthCheckRequest struct {
	HealthCheckEnabled bool                         `json:"health_check_enabled"`
	CheckInterval      int                          `json:"check_interval"`
	Timeout            int                          `json:"timeout"`
	UserAgent          string                       `json:"user_agent"`
	MaxRedirects       int                          `json:"max_redirects"`
	FollowRedirects    bool                         `json:"follow_redirects"`
	CheckFavicon       bool                         `json:"check_favicon"`
	HealthCheckConfig  *model.HealthCheckConfig     `json:"health_check_config" binding:"required"`
	HealthCheckAlert   *model.SiteHealthAlertConfig `json:"health_check_alert"`
	// ProbeKind selects a probe kind a plugin provides; empty keeps the
	// built-in check. ProbeConfig holds the values of its form.
	ProbeKind   string            `json:"probe_kind,omitempty"`
	ProbeConfig map[string]string `json:"probe_config,omitempty"`
}

type healthCheckSyncPayload struct {
	SiteKey    string                   `json:"site_key" binding:"required"`
	SiteName   string                   `json:"site_name" binding:"required"`
	Host       string                   `json:"host" binding:"required"`
	Port       int                      `json:"port"`
	Scheme     string                   `json:"scheme"`
	DisplayURL string                   `json:"display_url"`
	Config     updateHealthCheckRequest `json:"config" binding:"required"`
}

func validateHealthCheckRequest(req *updateHealthCheckRequest) error {
	if req.CheckInterval < 30 || req.CheckInterval > 3600 {
		return fmt.Errorf("check_interval must be between 30 and 3600 seconds")
	}
	if req.Timeout < 1 || req.Timeout > 60 {
		return fmt.Errorf("timeout must be between 1 and 60 seconds")
	}
	if req.MaxRedirects < 0 || req.MaxRedirects > 10 {
		return fmt.Errorf("max_redirects must be between 0 and 10")
	}
	if req.HealthCheckConfig == nil {
		return fmt.Errorf("health_check_config is required")
	}

	targetURL := strings.TrimSpace(req.HealthCheckConfig.TargetURL)
	if targetURL != "" {
		parsed, err := url.ParseRequestURI(targetURL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return fmt.Errorf("target_url must be an absolute URL without user information or a fragment")
		}
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https", "grpc", "grpcs":
		default:
			return fmt.Errorf("target_url scheme must be http, https, grpc, or grpcs")
		}
	}

	clickURL := strings.TrimSpace(req.HealthCheckConfig.ClickURL)
	if clickURL != "" {
		parsed, err := url.ParseRequestURI(clickURL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return fmt.Errorf("click_url must be an absolute URL without user information or a fragment")
		}
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
		default:
			return fmt.Errorf("click_url scheme must be http or https")
		}
	}

	if !sitecheck.IsValidProbeKind(req.ProbeKind) {
		return fmt.Errorf("probe_kind %q is not a valid probe kind", req.ProbeKind)
	}
	// The built-in check is stored as an empty kind, so a site that never
	// used a probe kind keeps its response shape.
	if sitecheck.IsBuiltinProbeKind(req.ProbeKind) {
		req.ProbeKind = ""
		req.ProbeConfig = nil
	}

	if req.HealthCheckAlert != nil {
		if req.HealthCheckAlert.FailureThreshold < 1 {
			req.HealthCheckAlert.FailureThreshold = 1
		}
		if req.HealthCheckAlert.CooldownSeconds < 0 {
			return fmt.Errorf("cooldown_seconds cannot be negative")
		}
	}
	return nil
}

func updateHealthCheckConfig(siteConfig *model.SiteConfig, req *updateHealthCheckRequest) error {
	if err := validateHealthCheckRequest(req); err != nil {
		return err
	}

	siteConfig.HealthCheckEnabled = req.HealthCheckEnabled
	siteConfig.CheckInterval = req.CheckInterval
	siteConfig.Timeout = req.Timeout
	siteConfig.UserAgent = req.UserAgent
	siteConfig.MaxRedirects = req.MaxRedirects
	siteConfig.FollowRedirects = req.FollowRedirects
	siteConfig.CheckFavicon = req.CheckFavicon
	siteConfig.HealthCheckConfig = req.HealthCheckConfig
	siteConfig.HealthCheckAlert = req.HealthCheckAlert
	siteConfig.ProbeKind = req.ProbeKind
	siteConfig.ProbeConfig = req.ProbeConfig

	// Select every field explicitly so false values are persisted while GORM's
	// model-field serializers still encode the JSON configuration columns.
	return model.UseDB().Model(siteConfig).Select(
		"HealthCheckEnabled",
		"CheckInterval",
		"Timeout",
		"UserAgent",
		"MaxRedirects",
		"FollowRedirects",
		"CheckFavicon",
		"HealthCheckConfig",
		"HealthCheckAlert",
		"ProbeKind",
		"ProbeConfig",
	).Updates(siteConfig).Error
}

func synchronizeHealthCheck(siteConfig *model.SiteConfig, req *updateHealthCheckRequest) []gin.H {
	nodes := internalsite.GetSyncNodes(siteConfig.SiteName)
	results := make([]gin.H, 0, len(nodes))
	remoteConfig := *req
	if req.HealthCheckAlert != nil {
		remoteAlert := *req.HealthCheckAlert
		// External notification records are node-local and may contain secrets.
		// Synchronize trigger semantics without copying local database IDs.
		remoteAlert.ExternalNotifyIDs = nil
		remoteConfig.HealthCheckAlert = &remoteAlert
	}
	payload := healthCheckSyncPayload{
		SiteKey:    siteConfig.SiteKey,
		SiteName:   siteConfig.SiteName,
		Host:       siteConfig.Host,
		Port:       siteConfig.Port,
		Scheme:     siteConfig.Scheme,
		DisplayURL: siteConfig.DisplayURL,
		Config:     remoteConfig,
	}

	for _, node := range nodes {
		client := nodeauth.NewRestyClient(node)
		client.SetBaseURL(node.URL)
		client.SetTimeout(10 * time.Second)
		resp, err := client.R().SetBody(payload).Put("/api/site_navigation/health_check/sync")
		result := gin.H{"node": node.Name, "success": false}
		if err != nil {
			result["error"] = err.Error()
		} else if resp.StatusCode() != http.StatusOK {
			result["status_code"] = resp.StatusCode()
			result["error"] = strings.TrimSpace(string(resp.Body()))
		} else {
			result["success"] = true
		}
		results = append(results, result)
	}
	return results
}

// UpdateHealthCheck updates local health-check configuration and propagates it
// to the nodes synchronized with the owning logical site.
func UpdateHealthCheck(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))
	var req updateHealthCheckRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	sc := query.SiteConfig
	siteConfig, err := sc.Where(sc.ID.Eq(id)).First()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// Restore before synchronizing too: the other nodes need the real values.
	req.ProbeConfig, err = restoreRedactedProbeConfig(siteConfig, req.ProbeKind, req.HealthCheckConfig, req.ProbeConfig)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := updateHealthCheckConfig(siteConfig, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	sitecheck.InvalidateSiteConfigCacheForHost(siteConfig.SiteKey)
	if service := sitecheck.GetService(); service != nil {
		service.RefreshSites()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Health check configuration updated successfully",
		"sync_results": synchronizeHealthCheck(siteConfig, &req),
	})
}

func SyncHealthCheck(c *gin.Context) {
	var payload healthCheckSyncPayload
	if !cosy.BindAndValid(c, &payload) {
		return
	}
	if err := validateHealthCheckRequest(&payload.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	db := model.UseDB()
	var siteConfig model.SiteConfig
	err := db.Where("site_key = ?", payload.SiteKey).First(&siteConfig).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		cosy.ErrHandler(c, err)
		return
	}
	if err == gorm.ErrRecordNotFound {
		siteConfig = model.SiteConfig{
			SiteKey:            payload.SiteKey,
			SiteName:           payload.SiteName,
			Host:               payload.Host,
			Port:               payload.Port,
			Scheme:             payload.Scheme,
			DisplayURL:         payload.DisplayURL,
			HealthCheckEnabled: true,
		}
		if err := db.Create(&siteConfig).Error; err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}
	if payload.Config.HealthCheckAlert != nil && siteConfig.HealthCheckAlert != nil && payload.Config.HealthCheckAlert.ExternalNotifyIDs == nil {
		payload.Config.HealthCheckAlert.ExternalNotifyIDs = append([]uint64(nil), siteConfig.HealthCheckAlert.ExternalNotifyIDs...)
	}

	if err := updateHealthCheckConfig(&siteConfig, &payload.Config); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	sitecheck.InvalidateSiteConfigCacheForHost(payload.SiteKey)
	if service := sitecheck.GetService(); service != nil {
		service.RefreshSites()
	}
	c.JSON(http.StatusOK, gin.H{"message": "Health check configuration synchronized successfully"})
}

// GetProbeKinds lists the probe kinds plugins offer next to the built-in
// check. The site check form shows a kind selector only when it is not empty.
func GetProbeKinds(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": sitecheck.ProbeKinds(),
	})
}

// TestHealthCheck tests a health check configuration without saving it
func TestHealthCheck(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var req struct {
		Config      *model.HealthCheckConfig `json:"config" binding:"required"`
		ProbeKind   string                   `json:"probe_kind"`
		ProbeConfig map[string]string        `json:"probe_config"`
	}

	if !cosy.BindAndValid(c, &req) {
		return
	}

	// Get site config to determine the host for testing
	sc := query.SiteConfig
	siteConfig, err := sc.Where(sc.ID.Eq(id)).First()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if !sitecheck.IsBuiltinProbeKind(req.ProbeKind) {
		probeConfig, err := restoreRedactedProbeConfig(siteConfig, req.ProbeKind, req.Config, req.ProbeConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		testProbeKind(c, siteConfig, req.Config, req.ProbeKind, probeConfig)
		return
	}

	// Create enhanced checker and test the configuration
	enhancedChecker := sitecheck.NewEnhancedSiteChecker()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass the stored site URL; CheckSiteWithConfig rewrites the scheme to match req.Config.Protocol.
	result, err := enhancedChecker.CheckSiteWithConfig(ctx, siteConfig.GetURL(), req.Config)

	if err != nil {
		logger.Errorf("Health check test failed for %s: %v", siteConfig.Host, err)
		c.JSON(http.StatusOK, gin.H{
			"success":       false,
			"error":         err.Error(),
			"response_time": 0,
		})
		return
	}

	info := result.Info
	if info == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":       false,
			"error":         "empty health check result",
			"response_time": 0,
		})
		return
	}

	success := info.Status == "online"
	errorMsg := ""
	errorType := ""
	if !success && info.Error != "" {
		errorMsg = info.Error
		errorType = info.ErrorType
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       success,
		"response_time": info.ResponseTime,
		"status":        info.Status,
		"status_code":   info.StatusCode,
		"error":         errorMsg,
		"error_type":    errorType,
	})
}

// testProbeKind runs one check with a plugin probe kind and answers in the
// shape of TestHealthCheck.
func testProbeKind(c *gin.Context, siteConfig *model.SiteConfig, config *model.HealthCheckConfig, kind string, probeConfig map[string]string) {
	if !sitecheck.IsValidProbeKind(kind) {
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("probe_kind %q is not a valid probe kind", kind)})
		return
	}

	target := siteConfig.GetURL()
	if config != nil && strings.TrimSpace(config.TargetURL) != "" {
		target = strings.TrimSpace(config.TargetURL)
	}
	timeout := time.Duration(siteConfig.Timeout) * time.Second

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result := sitecheck.TestProbe(ctx, kind, target, probeConfig, timeout)

	// A degraded target counts as reachable, the message is still shown.
	success := result.Status == sitecheck.StatusOnline
	c.JSON(http.StatusOK, gin.H{
		"success":       success,
		"response_time": result.ResponseTime,
		"status":        result.Status,
		"status_code":   result.StatusCode,
		"error":         result.Error,
		"error_type":    result.ErrorType,
	})
}
