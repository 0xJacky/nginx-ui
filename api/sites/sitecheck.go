package sites

import (
	"context"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
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
		"running": service.IsRunning(),
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
func updateSiteOrderBatchByIds(orderedIds []uint64) error {
	sc := query.SiteConfig

	for i, id := range orderedIds {
		if _, err := sc.Where(sc.ID.Eq(id)).Update(sc.CustomOrder, i); err != nil {
			return err
		}
	}

	return nil
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

// UpdateHealthCheck updates health check configuration for a site
func UpdateHealthCheck(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var req model.SiteConfig

	if !cosy.BindAndValid(c, &req) {
		return
	}

	sc := query.SiteConfig
	siteConfig, err := sc.Where(sc.ID.Eq(id)).First()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	siteConfig.HealthCheckEnabled = req.HealthCheckEnabled
	siteConfig.CheckInterval = req.CheckInterval
	siteConfig.Timeout = req.Timeout
	siteConfig.UserAgent = req.UserAgent
	siteConfig.MaxRedirects = req.MaxRedirects
	siteConfig.FollowRedirects = req.FollowRedirects
	siteConfig.CheckFavicon = req.CheckFavicon

	if req.HealthCheckConfig != nil {
		siteConfig.HealthCheckConfig = req.HealthCheckConfig
	}

	if err = query.SiteConfig.Save(siteConfig); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Health check configuration updated successfully",
	})
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

	// Convert host to URL for testing
	testURL := siteConfig.Scheme + "://" + siteConfig.Host
	result, err := enhancedChecker.CheckSiteWithConfig(ctx, testURL, req.Config)

	if err != nil {
		logger.Errorf("Health check test failed for %s: %v", siteConfig.Host, err)
		c.JSON(http.StatusOK, gin.H{
			"success":       false,
			"error":         err.Error(),
			"response_time": 0,
		})
		return
	}

	success := result.Status == "online"
	errorMsg := ""
	if !success && result.Error != "" {
		errorMsg = result.Error
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       success,
		"response_time": result.ResponseTime,
		"status":        result.Status,
		"status_code":   result.StatusCode,
		"error":         errorMsg,
	})
}
