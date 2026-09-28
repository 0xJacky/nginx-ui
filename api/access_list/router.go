package access_list

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers the access list endpoints.
func InitRouter(r *gin.RouterGroup) {
	r.GET("access_lists", GetAccessLists)
	r.GET("access_lists/:id", GetAccessList)
	r.GET("access_lists/:id/usage", GetAccessListUsage)
	r.POST("access_lists/preview", PreviewAccessList)

	// Detecting and rewriting access directives only transforms the content
	// the editor sends; nothing is written to disk.
	r.POST("access_control/state", GetAccessControlState)
	r.POST("access_control/apply", ApplyAccessControl)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("access_lists", CreateAccessList)
		o.POST("access_lists/:id", ModifyAccessList)
		o.DELETE("access_lists/:id", DeleteAccessList)
		o.POST("access_control/batch", BatchApplyAccessControl)
	}
}
