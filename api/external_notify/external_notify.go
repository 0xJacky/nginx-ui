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
	// Channels provided by plugins may reject a configuration before it is
	// stored; the built-in ones are not checked here.
	c.CreateHook(validateNotifierConfig)
	c.ModifyHook(validateNotifierConfig)
	c.InitRouter(r)

	// Notifier types offered next to the built-in ones, with the schema of
	// their configuration form.
	r.GET("/external_notifies/channels", listChannels)

	// Sending a test message posts to whatever endpoint the caller supplies.
	r.POST(
		"/external_notifies/test",
		middleware.RequireInteractiveUser(),
		middleware.RequireSecureSession(),
		middleware.RejectInDemo(),
		testMessage,
	)
}

// listChannels returns the notifier types provided by plugins.
func listChannels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": notification.ExternalNotifierChannels(),
	})
}

// validateNotifierConfig lets the source of a plugin notifier type check the
// configuration when a record is created or its type or config changes.
func validateNotifierConfig(ctx *cosy.Ctx[model.ExternalNotify]) {
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.ExternalNotify]) {
		_, hasType := ctx.Payload["type"]
		_, hasConfig := ctx.Payload["config"]
		if !hasType && !hasConfig {
			return
		}

		notifierType := ctx.Model.Type
		if !hasType {
			notifierType = ctx.OriginModel.Type
		}
		config := ctx.Model.Config
		if !hasConfig {
			config = ctx.OriginModel.Config
		}

		validateCtx, cancel := context.WithTimeout(ctx.RequestContext(), externalNotifyTestTimeout)
		defer cancel()
		if err := notification.ValidateExternalNotifierConfig(validateCtx, notifierType, config); err != nil {
			ctx.AbortWithError(err)
		}
	})
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
