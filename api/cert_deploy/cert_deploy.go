// Package cert_deploy serves the deploy targets certificates are pushed to
// and the outcomes of the pushes.
package cert_deploy

import (
	"context"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/deploy"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

const (
	// validateTimeout bounds the configuration check on save.
	validateTimeout = 30 * time.Second
	// deploymentsLimit bounds the history one request returns.
	deploymentsLimit = 50
)

func InitRouter(r *gin.RouterGroup) {
	// Changing a target decides where private keys are sent, so it needs an
	// interactive administrator with a secure session, and stays closed in
	// demo mode.
	mutationMiddleware := []gin.HandlerFunc{
		middleware.RequireInteractiveUser(),
		middleware.RequireSecureSession(),
		middleware.RejectInDemo(),
	}

	c := cosy.Api[model.CertDeployTarget]("/cert_deploy_targets")
	c.BeforeCreate(mutationMiddleware...).
		BeforeModify(mutationMiddleware...).
		BeforeDestroy(mutationMiddleware...).
		BeforeRecover(mutationMiddleware...)
	c.GetListHook(func(ctx *cosy.Ctx[model.CertDeployTarget]) {
		ctx.SetFussy("name").SetEqual("kind", "cert_id", "enabled")
		redactForServiceTokens(ctx)
	})
	c.GetHook(redactForServiceTokens)
	c.CreateHook(validateTarget)
	c.ModifyHook(validateTarget)
	c.InitRouter(r)

	// Target kinds enabled plugins offer, with the schema of their form.
	r.GET("/cert_deploy_targets/kinds", listKinds)
	r.GET("/cert_deploy_targets/:id/deployments", listDeployments)
	r.GET("/certs/:id/deploy_targets", listCertificateTargets)

	// Pushing sends private keys to an external target.
	o := r.Group("", mutationMiddleware...)
	{
		o.POST("/cert_deploy_targets/test", testTarget)
		o.POST("/cert_deploy_targets/:id/deploy", deployTarget)
		o.POST("/certs/:id/deploy", deployCertificate)
	}
}

// redactedTarget is a target without its configuration values.
type redactedTarget struct {
	model.Model
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	CertID  uint64 `json:"cert_id"`
	Enabled bool   `json:"enabled"`
}

func redactTarget(target *model.CertDeployTarget) redactedTarget {
	return redactedTarget{
		Model:   target.Model,
		Name:    target.Name,
		Kind:    target.Kind,
		CertID:  target.CertID,
		Enabled: target.Enabled,
	}
}

// redactForServiceTokens keeps the credentials of a target away from MCP
// service tokens, which only read.
func redactForServiceTokens(ctx *cosy.Ctx[model.CertDeployTarget]) {
	if internalmcp.IsServiceTokenRequest(ctx.Context) {
		ctx.SetTransformer(redactTarget)
	}
}

// validateTarget checks the certificate binding and lets the plugin behind
// the kind check the configuration when a target is created or its kind or
// values change. The payload may carry only some fields on modify, so the
// stored ones fill the gaps.
func validateTarget(ctx *cosy.Ctx[model.CertDeployTarget]) {
	ctx.BeforeExecuteHook(func(ctx *cosy.Ctx[model.CertDeployTarget]) {
		if _, hasCert := ctx.Payload["cert_id"]; hasCert && ctx.Model.CertID != 0 {
			if _, err := query.Cert.Where(query.Cert.ID.Eq(ctx.Model.CertID)).First(); err != nil {
				ctx.AbortWithError(err)
				return
			}
		}

		_, hasKind := ctx.Payload["kind"]
		_, hasConfig := ctx.Payload["config"]
		if !hasKind && !hasConfig {
			return
		}
		kind := ctx.Model.Kind
		if !hasKind {
			kind = ctx.OriginModel.Kind
		}
		config := ctx.Model.Config
		if !hasConfig {
			config = ctx.OriginModel.Config
		}

		validateCtx, cancel := context.WithTimeout(ctx.RequestContext(), validateTimeout)
		defer cancel()
		if err := deploy.ValidateConfig(validateCtx, kind, config); err != nil {
			ctx.AbortWithError(err)
		}
	})
}

// listKinds returns the target kinds enabled cert.deploy plugins offer.
func listKinds(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": deploy.Kinds()})
}

// listDeployments returns the latest outcomes of a target, newest first,
// optionally of one certificate.
func listDeployments(c *gin.Context) {
	deployments, err := deploy.Deployments(cast.ToUint64(c.Param("id")), cast.ToUint64(c.Query("cert_id")), deploymentsLimit)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": deployments})
}

// certificateTarget is a target bound to a certificate with its latest
// outcome for that certificate.
type certificateTarget struct {
	Target redactedTarget `json:"target"`
	// KindName is the display name of the kind, empty when no enabled
	// plugin offers it right now.
	KindName string                `json:"kind_name,omitempty"`
	Last     *model.CertDeployment `json:"last,omitempty"`
}

// listCertificateTargets returns every target bound to a certificate with
// its latest outcome.
func listCertificateTargets(c *gin.Context) {
	certID := cast.ToUint64(c.Param("id"))
	targets, err := deploy.TargetsOf(certID)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	kindNames := map[string]string{}
	for _, kind := range deploy.Kinds() {
		kindNames[kind.Kind] = kind.Name
	}

	list := make([]certificateTarget, 0, len(targets))
	for _, target := range targets {
		list = append(list, certificateTarget{
			Target:   redactTarget(target),
			KindName: kindNames[target.Kind],
			Last:     deploy.LatestDeployment(target.ID, certID),
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// testTargetRequest is a target configuration to try against a certificate.
type testTargetRequest struct {
	Kind   string            `json:"kind" binding:"required"`
	Config map[string]string `json:"config"`
	// CertID picks the certificate, 0 picks the first one nginx serves.
	CertID uint64 `json:"cert_id"`
}

// testTarget runs a dry run of a target configuration: the plugin checks the
// target without changing anything. Nothing is recorded.
func testTarget(c *gin.Context) {
	var req testTargetRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	if err := deploy.ValidateConfig(c.Request.Context(), req.Kind, req.Config); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	message, err := deploy.DryRun(c.Request.Context(), req.Kind, req.Config, req.CertID)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": message})
}

// deployTarget pushes every certificate bound to a target once.
func deployTarget(c *gin.Context) {
	results, err := deploy.DeployTarget(c.Request.Context(), cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

// deployCertificate pushes a certificate to every enabled target bound to
// it once.
func deployCertificate(c *gin.Context) {
	results, err := deploy.DeployCertificate(c.Request.Context(), cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}
