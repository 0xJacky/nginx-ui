package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/api/cluster"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// With the full router layout a request that selects a remote node reaches
// the controller's own relationship endpoints, which accept only a node
// principal, so it is refused locally and never forwarded.
func TestRelationshipEndpointsRefuseRemoteNodeSelection(t *testing.T) {
	f := setupRelationshipFixture(t, router.NodeRouters)
	assertForwardedRelationshipCallsRefused(t, f, func(t *testing.T, res *httptest.ResponseRecorder) {
		require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	})

	// The same callers are refused when they address the controller itself.
	for _, caller := range f.callers() {
		req := httptest.NewRequest(http.MethodPost, "/api/node/credentials/"+relationshipCredentialID+"/rotation/confirm", nil)
		req.Header.Set("Authorization", "Bearer "+caller.token)
		if caller.secureSession != "" {
			req.Header.Set("X-Secure-Session-ID", caller.secureSession)
		}
		res := httptest.NewRecorder()
		f.controller.ServeHTTP(res, req)
		require.Equal(t, http.StatusForbidden, res.Code, "%s: %s", caller.name, res.Body.String())
	}
}

// Mounting the relationship endpoints behind Proxy by mistake still keeps
// them off the proxy: Proxy does not know RequireNodePrincipal and refuses the
// route with 40302 instead of forwarding it.
func TestRelationshipEndpointsBehindProxyAreRefused(t *testing.T) {
	f := setupRelationshipFixture(t, router.NodeRouters)
	misconfigured := gin.New()
	cluster.InitNodeRouter(misconfigured.Group("/api", middleware.AuthRequired(), middleware.Proxy()))

	caller := f.callers()[1]
	f.childNodeRequests.Store(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/node/credentials/"+relationshipCredentialID+"/rotation/confirm", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+caller.token)
	req.Header.Set("X-Secure-Session-ID", caller.secureSession)
	req.Header.Set("X-Node-ID", strconv.FormatUint(f.pairedNodeID, 10))
	res := httptest.NewRecorder()
	misconfigured.ServeHTTP(res, req)
	require.Equal(t, http.StatusServiceUnavailable, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), `"code":40302`)
	require.Zero(t, f.childNodeRequests.Load())
}

// The controller reaches the relationship endpoints of a child directly,
// signed as itself: the legacy upgrade and manual and automatic rotation.
func TestRelationshipFlowsReachChildDirectly(t *testing.T) {
	f := setupRelationshipFixture(t, router.NodeRouters)
	ctx := context.Background()
	// The fixture shares one database between controller and child, and an
	// upgrade revokes every credential of the instance it names, so the
	// upgrade runs as a different controller instance than the paired node.
	const upgradingInstanceID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	loadNode := func(id uint64) *model.Node {
		var node model.Node
		require.NoError(t, f.db.First(&node, id).Error)
		return &node
	}
	childKey := func(credentialID string) []byte {
		var credential model.NodeControllerCredential
		require.NoError(t, f.db.Where("credential_id = ? AND revoked_at IS NULL", credentialID).First(&credential).Error)
		require.Equal(t, model.NodeCredentialStatusActive, credential.Status)
		return credential.PublicKey
	}
	controllerKey := func(nodeID uint64) model.NodeCredential {
		var credential model.NodeCredential
		require.NoError(t, f.db.Where("node_id = ? AND revoked_at IS NULL", nodeID).First(&credential).Error)
		return credential
	}

	t.Run("legacy upgrade", func(t *testing.T) {
		f.childNodeRequests.Store(0)
		result, err := nodeauth.UpgradeLegacyRelationship(ctx, loadNode(f.legacyNodeID), upgradingInstanceID)
		require.NoError(t, err)
		require.Equal(t, relationshipChildInstanceID, result.TargetInstanceID)
		require.Equal(t, int32(1), f.childNodeRequests.Load())
		node := loadNode(f.legacyNodeID)
		require.Equal(t, model.NodeAuthMethodPaired, node.AuthMethod)
		require.Equal(t, controllerKey(f.legacyNodeID).PublicKey, childKey(result.CredentialID))
	})

	t.Run("manual rotation", func(t *testing.T) {
		before := controllerKey(f.pairedNodeID).PublicKey
		f.childNodeRequests.Store(0)
		result, err := nodeauth.RotateRelationshipCredential(ctx, loadNode(f.pairedNodeID))
		require.NoError(t, err)
		require.Equal(t, int32(2), f.childNodeRequests.Load(), "begin and confirm")
		after := controllerKey(f.pairedNodeID).PublicKey
		require.NotEqual(t, before, after)
		require.Equal(t, after, childKey(result.CredentialID))
	})

	t.Run("automatic rotation", func(t *testing.T) {
		before := controllerKey(f.pairedNodeID)
		due := time.Now().Add(-91 * 24 * time.Hour)
		require.NoError(t, f.db.Model(&model.NodeCredential{}).Where("id = ?", before.ID).Update("rotated_at", due).Error)
		require.Empty(t, nodeauth.MaintainRelationships(ctx, upgradingInstanceID, time.Now()))
		after := controllerKey(f.pairedNodeID).PublicKey
		require.NotEqual(t, before.PublicKey, after)
		require.Equal(t, after, childKey(before.CredentialID))
	})
}

// RequireNodePrincipal lets only an authenticated node principal through.
func TestRequireNodePrincipalRefusesOtherCallers(t *testing.T) {
	engine := gin.New()
	engine.POST("/probe", func(c *gin.Context) {
		if c.GetHeader("X-Test-Node") != "" {
			c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{CredentialID: "legacy", AuthMethod: model.NodeAuthMethodLegacy})
		}
		c.Next()
	}, middleware.RequireNodePrincipal(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	res := httptest.NewRecorder()
	engine.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/probe", nil))
	require.Equal(t, http.StatusForbidden, res.Code)
	require.True(t, strings.Contains(res.Body.String(), "node controller"), res.Body.String())

	req := httptest.NewRequest(http.MethodPost, "/probe", nil)
	req.Header.Set("X-Test-Node", "1")
	res = httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	require.Equal(t, http.StatusNoContent, res.Code)
}
