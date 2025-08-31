package license

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"

	"github.com/0xJacky/Nginx-UI/internal/license"
)

type Controller struct{}

func InitRouter(r *gin.RouterGroup) {
	c := NewController()

	licenseGroup := r.Group("/licenses")
	{
		licenseGroup.GET("", c.GetLicenses)
		licenseGroup.GET("/backend", c.GetBackendLicenses)
		licenseGroup.GET("/frontend", c.GetFrontendLicenses)
		licenseGroup.GET("/stats", c.GetLicenseStats)
	}
}

func NewController() *Controller {
	return &Controller{}
}

// GetLicenses godoc
// @Summary Get all open source component licenses
// @Description Returns license information for all backend and frontend components
// @Tags License
// @Accept json
// @Produce json
// @Success 200 {object} license.ComponentInfo "License information"
// @Failure 500 {object} cosy.HTTPError "Internal Server Error"
// @Router /api/licenses [get]
func (c *Controller) GetLicenses(ctx *gin.Context) {
	info, err := license.GetLicenseInfo()
	if err != nil {
		cosy.ErrHandler(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, info)
}

// GetBackendLicenses godoc
// @Summary Get backend component licenses
// @Description Returns license information for backend Go modules
// @Tags License
// @Accept json
// @Produce json
// @Success 200 {array} license.License "Backend license information"
// @Failure 500 {object} cosy.HTTPError "Internal Server Error"
// @Router /api/licenses/backend [get]
func (c *Controller) GetBackendLicenses(ctx *gin.Context) {
	licenses, err := license.GetBackendLicenses()
	if err != nil {
		cosy.ErrHandler(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, licenses)
}

// GetFrontendLicenses godoc
// @Summary Get frontend component licenses
// @Description Returns license information for frontend npm packages
// @Tags License
// @Accept json
// @Produce json
// @Success 200 {array} license.License "Frontend license information"
// @Failure 500 {object} cosy.HTTPError "Internal Server Error"
// @Router /api/licenses/frontend [get]
func (c *Controller) GetFrontendLicenses(ctx *gin.Context) {
	licenses, err := license.GetFrontendLicenses()
	if err != nil {
		cosy.ErrHandler(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, licenses)
}

// GetLicenseStats godoc
// @Summary Get license statistics
// @Description Returns statistics about the distribution of licenses
// @Tags License
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "License statistics"
// @Failure 500 {object} cosy.HTTPError "Internal Server Error"
// @Router /api/licenses/stats [get]
func (c *Controller) GetLicenseStats(ctx *gin.Context) {
	stats, err := license.GetLicenseStats()
	if err != nil {
		cosy.ErrHandler(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, stats)
}
