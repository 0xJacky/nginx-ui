package external_notify

import (
	"context"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

const externalNotifyTestTimeout = 10 * time.Second

func InitRouter(r *gin.RouterGroup) {
	c := cosy.Api[model.ExternalNotify]("/external_notifies")
	mutationMiddleware := []gin.HandlerFunc{
		middleware.RequireInteractiveUser(),
		middleware.RequireSecureSession(),
		middleware.RejectInDemo(),
	}

	// Reads remain available to authenticated API clients, while changing a
	// notifier requires an interactive administrator because its configuration
	// controls an outbound request destination.
	c.BeforeCreate(mutationMiddleware...).
		BeforeModify(mutationMiddleware...).
		BeforeDestroy(mutationMiddleware...).
		BeforeRecover(mutationMiddleware...)
	c.InitRouter(r)

	// Sending a test message posts to whatever endpoint the caller supplies.
	r.POST(
		"/external_notifies/test",
		middleware.RequireInteractiveUser(),
		middleware.RequireSecureSession(),
		middleware.RejectInDemo(),
		testMessage,
	)
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), externalNotifyTestTimeout)
	defer cancel()

	err := notification.SendTestMessageContext(ctx, req.Type, req.Language, req.Config)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
