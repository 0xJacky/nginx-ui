package external_notify

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func InitRouter(r *gin.RouterGroup) {
	c := cosy.Api[model.ExternalNotify]("/external_notifies")

	// Reads stay open so the list is worth showing, but creating an enabled
	// notifier means storing an arbitrary outbound endpoint that the
	// notification system will genuinely post to.
	c.InitRouter(r.Group("", middleware.DemoReadOnly()))

	// Sending a test message posts to whatever endpoint the caller supplies.
	r.POST("/external_notifies/test", middleware.RejectInDemo(), testMessage)
}

// testMessage sends a test message with direct parameters
func testMessage(c *gin.Context) {
	var req struct {
		Type     string            `json:"type" binding:"required"`
		Language string            `json:"language" binding:"required"`
		Config   map[string]string `json:"config" binding:"required"`
	}
	if !cosy.BindAndValid(c, &req) {
		return
	}

	// Send test notification with direct parameters
	err := notification.SendTestMessage(req.Type, req.Language, req.Config)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
