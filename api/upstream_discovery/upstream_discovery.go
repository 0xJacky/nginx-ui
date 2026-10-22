// Package upstream_discovery serves the upstream discoveries, which bind an
// nginx upstream to a service an upstream.discovery plugin resolves.
package upstream_discovery

import (
	"net/http"
	"strconv"
	"strings"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/upstream/discovery"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func InitRouter(r *gin.RouterGroup) {
	// A binding decides where nginx proxies to, so changing one needs an
	// interactive administrator with a secure session and stays closed in
	// demo mode.
	mutationMiddleware := []gin.HandlerFunc{
		middleware.RequireInteractiveUser(),
		middleware.RequireSecureSession(),
		middleware.RejectInDemo(),
	}

	c := cosy.Api[model.UpstreamDiscovery]("/upstream_discoveries")
	c.BeforeCreate(mutationMiddleware...).
		BeforeModify(mutationMiddleware...).
		BeforeDestroy(mutationMiddleware...).
		BeforeRecover(mutationMiddleware...)
	c.GetListHook(func(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
		ctx.SetFussy("upstream_name", "service").SetEqual("kind", "enabled")
		setView(ctx)
	})
	c.GetHook(setView)
	c.CreateHook(func(ctx *cosy.Ctx[model.UpstreamDiscovery]) { validateBinding(ctx, true) })
	c.ModifyHook(func(ctx *cosy.Ctx[model.UpstreamDiscovery]) { validateBinding(ctx, false) })
	c.DestroyHook(removeFile)
	c.RecoverHook(scheduleAgain)
	c.InitRouter(r)

	// Providers enabled plugins offer, with the schema of their form.
	r.GET("/upstream_discoveries/kinds", listProviders)

	o := r.Group("", mutationMiddleware...)
	{
		o.POST("/upstream_discoveries/:id/refresh", refreshBinding)
	}
}

// bindingView is a binding with the file it is written to and the include
// line a person adds to the http block.
type bindingView struct {
	*model.UpstreamDiscovery
	// Config shadows the stored values, so they can be left out.
	Config map[string]string `json:"config,omitempty"`
	// Include is the directive to add to the http block.
	Include string `json:"include"`
	// Path is the absolute path of the file.
	Path string `json:"path"`
}

func viewOf(binding *model.UpstreamDiscovery, redact bool) bindingView {
	view := bindingView{
		UpstreamDiscovery: binding,
		Config:            binding.Config,
		Include:           discovery.IncludeLine(binding.UpstreamName),
		Path:              discovery.Default().Path(binding.UpstreamName),
	}
	if redact {
		view.Config = nil
	}
	return view
}

// setView adds the file to every binding and keeps the credentials of a
// binding away from MCP service tokens, which only read.
func setView(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
	redact := internalmcp.IsServiceTokenRequest(ctx.Context)
	ctx.SetTransformer(func(binding *model.UpstreamDiscovery) bindingView { return viewOf(binding, redact) })
}

// validateBinding checks the upstream name, the provider, the required
// fields, the interval and the extra directives when a binding is created or
// changed. The payload may carry only some fields on modify, so the stored
// ones fill the gaps. Renaming the upstream removes the file of the old name
// first. A change that affects the upstream makes the binding due at once.
func validateBinding(ctx *cosy.Ctx[model.UpstreamDiscovery], creating bool) {
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
		payload := func(key string) bool {
			_, ok := ctx.Payload[key]
			return ok
		}
		hasName, hasKind, hasConfig := payload("upstream_name"), payload("kind"), payload("config")
		hasRefresh, hasExtra := payload("refresh_seconds"), payload("extra_directives")

		if creating || hasName {
			ctx.Model.UpstreamName = strings.TrimSpace(ctx.Model.UpstreamName)
			name := ctx.Model.UpstreamName
			if !discovery.IsValidUpstreamName(name) {
				ctx.AbortWithError(cosy.WrapErrorWithParams(plugin.ErrUpstreamNameInvalid, name))
				return
			}
			q := query.UpstreamDiscovery
			taken, err := q.Where(q.UpstreamName.Eq(name), q.ID.Neq(ctx.OriginModel.ID)).Count()
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			if taken > 0 {
				ctx.AbortWithError(cosy.WrapErrorWithParams(plugin.ErrUpstreamNameTaken, name))
				return
			}
		}

		kind := ctx.Model.Kind
		if !hasKind && !creating {
			kind = ctx.OriginModel.Kind
		}
		config := ctx.Model.Config
		if !hasConfig && !creating {
			config = ctx.OriginModel.Config
		}
		if creating || hasKind || hasConfig {
			if err := discovery.ValidateConfig(kind, config); err != nil {
				ctx.AbortWithError(err)
				return
			}
		}

		if creating && ctx.Model.RefreshSeconds == 0 {
			ctx.Model.RefreshSeconds = discovery.DefaultRefreshSeconds
		}
		if (creating || hasRefresh) && ctx.Model.RefreshSeconds < discovery.MinRefreshSeconds {
			ctx.AbortWithError(cosy.WrapErrorWithParams(plugin.ErrRefreshIntervalInvalid, strconv.Itoa(discovery.MinRefreshSeconds)))
			return
		}
		if (creating || hasExtra) && !discovery.IsValidExtraDirectives(ctx.Model.ExtraDirectives) {
			ctx.AbortWithError(plugin.ErrExtraDirectivesInvalid)
			return
		}

		if creating {
			return
		}
		if hasName && ctx.Model.UpstreamName != ctx.OriginModel.UpstreamName {
			if err := discovery.Default().Remove(ctx.OriginModel.UpstreamName); err != nil {
				ctx.AbortWithError(err)
				return
			}
		}
		if len(ctx.Payload) > 0 {
			ctx.Model.NextRunAt = nil
			ctx.AddSelectedFields("next_run_at")
		}
	})
}

// removeFile deletes the file of a binding before the binding goes. It is
// refused while nginx still includes the file or proxies to the upstream.
func removeFile(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
		if err := discovery.Default().Remove(ctx.OriginModel.UpstreamName); err != nil {
			ctx.AbortWithError(err)
		}
	})
}

// scheduleAgain makes a recovered binding due at once, which writes its file
// back. A binding whose upstream name was taken meanwhile stays deleted.
func scheduleAgain(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
	// The recover chain runs no executed hooks, so the row is updated
	// before it is restored.
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.UpstreamDiscovery]) {
		q := query.UpstreamDiscovery
		taken, err := q.Where(q.UpstreamName.Eq(ctx.Model.UpstreamName), q.ID.Neq(ctx.Model.ID)).Count()
		if err != nil {
			ctx.AbortWithError(err)
			return
		}
		if taken > 0 {
			ctx.AbortWithError(cosy.WrapErrorWithParams(plugin.ErrUpstreamNameTaken, ctx.Model.UpstreamName))
			return
		}
		if _, err = q.Unscoped().Where(q.ID.Eq(ctx.Model.ID)).UpdateColumnSimple(q.NextRunAt.Null()); err != nil {
			ctx.AbortWithError(err)
		}
	})
}

// listProviders returns the providers enabled upstream.discovery plugins
// offer.
func listProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": discovery.Providers()})
}

// refreshBinding resolves a binding now and returns it with the outcome.
func refreshBinding(c *gin.Context) {
	binding, err := discovery.Default().Refresh(c.Request.Context(), cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, viewOf(binding, false))
}
