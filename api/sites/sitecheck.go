package sites

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	sites := service.GetSites()

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

	c.JSON(http.StatusOK, siteConfig)
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

	// Select every field explicitly so false values are persisted while GORM's
	// model-field serializers still encode the two JSON configuration columns.
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

// TestHealthCheck tests a health check configuration without saving it
func TestHealthCheck(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var req struct {
		Config *model.HealthCheckConfig `json:"config" binding:"required"`
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
