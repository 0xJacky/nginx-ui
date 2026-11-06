// Package blocklist serves the blocklist sources, whose lists of addresses
// security.blocklist plugins fetch and nginx-ui writes as deny rules.
package blocklist

import (
	"net/http"
	"strconv"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	internalblocklist "github.com/0xJacky/Nginx-UI/internal/security/blocklist"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func InitRouter(r *gin.RouterGroup) {
	// A source decides which clients nginx denies, so changing one needs an
	// interactive administrator with a secure session and stays closed in
	// demo mode.
	mutationMiddleware := []gin.HandlerFunc{
		middleware.RequireInteractiveUser(),
		middleware.RequireSecureSession(),
		middleware.RejectInDemo(),
	}

	c := cosy.Api[model.BlocklistSource]("/blocklist_sources")
	c.BeforeCreate(mutationMiddleware...).
		BeforeModify(mutationMiddleware...).
		BeforeDestroy(mutationMiddleware...).
		BeforeRecover(mutationMiddleware...)
	c.GetListHook(func(ctx *cosy.Ctx[model.BlocklistSource]) {
		ctx.SetFussy("name").SetEqual("kind", "enabled")
		setView(ctx)
	})
	c.GetHook(setView)
	c.CreateHook(func(ctx *cosy.Ctx[model.BlocklistSource]) { validateSource(ctx, true) })
	c.ModifyHook(func(ctx *cosy.Ctx[model.BlocklistSource]) { validateSource(ctx, false) })
	c.DestroyHook(removeFile)
	c.RecoverHook(scheduleAgain)
	c.InitRouter(r)

	// Source kinds enabled plugins offer, with the schema of their form.
	r.GET("/blocklist_sources/kinds", listKinds)

	o := r.Group("", mutationMiddleware...)
	{
		o.POST("/blocklist_sources/:id/refresh", refreshSource)
	}
}

// sourceView is a source with the file it is written to and the include line
// a person adds where the list should apply.
type sourceView struct {
	*model.BlocklistSource
	// Config shadows the stored values, so they can be left out.
	Config map[string]string `json:"config,omitempty"`
	// Include is the directive to add to a server or location block.
	Include string `json:"include"`
	// Path is the absolute path of the file.
	Path string `json:"path"`
}

func viewOf(source *model.BlocklistSource, redact bool) sourceView {
	view := sourceView{
		BlocklistSource: source,
		Config:          source.Config,
		Include:         internalblocklist.IncludeLine(source.ID),
		Path:            internalblocklist.Default().Path(source.ID),
	}
	if redact {
		view.Config = nil
	}
	return view
}

// setView adds the file to every source and keeps the credentials of a
// source away from MCP service tokens, which only read.
func setView(ctx *cosy.Ctx[model.BlocklistSource]) {
	redact := internalmcp.IsServiceTokenRequest(ctx.Context)
	ctx.SetTransformer(func(source *model.BlocklistSource) sourceView { return viewOf(source, redact) })
}

// validateSource checks the kind, the required fields and the interval when
// a source is created or changed. The payload may carry only some fields on
// modify, so the stored ones fill the gaps. A change that affects the list
// makes the source due at once.
func validateSource(ctx *cosy.Ctx[model.BlocklistSource], creating bool) {
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.BlocklistSource]) {
		_, hasKind := ctx.Payload["kind"]
		_, hasConfig := ctx.Payload["config"]
		_, hasRefresh := ctx.Payload["refresh_seconds"]
		_, hasEnabled := ctx.Payload["enabled"]

		kind := ctx.Model.Kind
		if !hasKind && !creating {
			kind = ctx.OriginModel.Kind
		}
		config := ctx.Model.Config
		if !hasConfig && !creating {
			config = ctx.OriginModel.Config
		}

		if creating || hasKind || hasConfig {
			if err := internalblocklist.ValidateConfig(kind, config); err != nil {
				ctx.AbortWithError(err)
				return
			}
		}

		if creating && ctx.Model.RefreshSeconds == 0 {
			ctx.Model.RefreshSeconds = protocol.DefaultBlocklistRefreshSeconds
			if offered, ok := internalblocklist.KindOf(kind); ok && offered.RefreshSeconds > 0 {
				ctx.Model.RefreshSeconds = offered.RefreshSeconds
			}
		}
		if (creating || hasRefresh) && ctx.Model.RefreshSeconds < protocol.MinBlocklistRefreshSeconds {
			ctx.AbortWithError(cosy.WrapErrorWithParams(plugin.ErrRefreshIntervalInvalid, strconv.Itoa(protocol.MinBlocklistRefreshSeconds)))
			return
		}

		if !creating && (hasKind || hasConfig || hasRefresh || hasEnabled) {
			ctx.Model.NextRunAt = nil
			ctx.AddSelectedFields("next_run_at")
		}
	})
}

// removeFile deletes the file of a source before the source goes. It is
// refused while nginx still includes the file.
func removeFile(ctx *cosy.Ctx[model.BlocklistSource]) {
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.BlocklistSource]) {
		if err := internalblocklist.Default().Remove(ctx.OriginModel.ID); err != nil {
			ctx.AbortWithError(err)
		}
	})
}

// scheduleAgain makes a recovered source due at once, which writes its file
// back.
func scheduleAgain(ctx *cosy.Ctx[model.BlocklistSource]) {
	// The recover chain runs no executed hooks, so the row is updated
	// before it is restored.
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.BlocklistSource]) {
		q := query.BlocklistSource
		if _, err := q.Unscoped().Where(q.ID.Eq(ctx.Model.ID)).UpdateColumnSimple(q.NextRunAt.Null()); err != nil {
			ctx.AbortWithError(err)
		}
	})
}

// listKinds returns the source kinds enabled security.blocklist plugins
// offer.
func listKinds(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": internalblocklist.Kinds()})
}

// refreshSource fetches a source now and returns it with the outcome.
func refreshSource(c *gin.Context) {
	source, err := internalblocklist.Default().Refresh(c.Request.Context(), cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, viewOf(source, false))
}
