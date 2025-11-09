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

// UpdateHealthCheck updates health check configuration for a site
func UpdateHealthCheck(c *gin.Context) {
	cosy.Core[model.SiteConfig](c).Modify()
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
