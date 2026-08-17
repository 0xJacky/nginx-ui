package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNodeResponseRedactsAllCredentialMaterial(t *testing.T) {
	node := &model.Node{
		Model:                 model.Model{ID: 77},
		Name:                  "redacted-node",
		URL:                   "https://node.example",
		Token:                 "plaintext-secret",
		EncryptedLegacySecret: []byte("encrypted-secret"),
		AuthMethod:            model.NodeAuthMethodLegacy,
		CredentialStatus:      model.NodeCredentialStatusActive,
	}

	encoded, err := json.Marshal(newNodeResponse(node))
	require.NoError(t, err)
	response := string(encoded)
	require.NotContains(t, response, "plaintext-secret")
	require.NotContains(t, response, "encrypted-secret")
	require.NotContains(t, response, "token")
	require.NotContains(t, response, "private_key")
	require.Contains(t, response, `"auth_method":"legacy_secret"`)
	require.Contains(t, response, `"auth_upgrade_status":"paused"`)
	require.Contains(t, response, `"has_credential":true`)
	require.Contains(t, response, `"status":false`)
	require.NotContains(t, response, `"NodeStat"`)
	// The edit form needs to know a secret exists without receiving it.
	require.Contains(t, response, `"legacy_secret":"`+settings.RedactedSensitiveValue+`"`)

	withoutSecret, err := json.Marshal(newNodeResponse(&model.Node{Model: model.Model{ID: 78}}))
	require.NoError(t, err)
	require.NotContains(t, string(withoutSecret), "legacy_secret")
}

// TestSubmittedSecretIgnoresTheRedactionSentinel pins the round trip: an edit
// that leaves the field untouched sends the sentinel back, and storing it
// literally would replace the node's real secret with a useless string.
func TestSubmittedSecretIgnoresTheRedactionSentinel(t *testing.T) {
	sentinel := settings.RedactedSensitiveValue
	actual := "  a-real-secret  "

	require.Empty(t, mutationLegacySecret(nodeMutationRequest{LegacySecret: &sentinel}))
	require.Empty(t, mutationLegacySecret(nodeMutationRequest{Token: &sentinel}))
	require.Equal(t, "a-real-secret", mutationLegacySecret(nodeMutationRequest{LegacySecret: &actual}))
	require.Empty(t, mutationLegacySecret(nodeMutationRequest{}))
}

func TestGetNodeSecretRequiresAVerifiedSecureSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/nodes/:id/secret", GetNodeSecret)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/nodes/1/secret", nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func setupNodeLifecycleTest(t *testing.T) *gorm.DB {
	t.Helper()
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Node{}, model.NodeCredential{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "node-delete.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)
	return db
}

func TestDeleteNodeReloadsStatusAfterSuccessfulDeletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNodeLifecycleTest(t)

	node := &model.Node{
		Name:    "deleted-node",
		URL:     "http://127.0.0.1:1",
		Token:   "test-token",
		Enabled: true,
	}
	require.NoError(t, db.Create(node).Error)

	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		analytic.RetrieveNodesStatus(monitorCtx)
	}()
	t.Cleanup(func() {
		cancelMonitor()
		select {
		case <-monitorDone:
		case <-time.After(time.Second):
			t.Error("node status monitor did not stop")
		}
	})

	require.Eventually(t, func() bool {
		_, exists := analytic.SnapshotNodeMap()[node.ID]
		return exists
	}, time.Second, 10*time.Millisecond)

	router := gin.New()
	router.DELETE("/nodes/:id", DeleteNode)
	request := httptest.NewRequest(http.MethodDelete, "/nodes/"+strconv.FormatUint(node.ID, 10), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.ErrorIs(t, db.First(&model.Node{}, node.ID).Error, gorm.ErrRecordNotFound)
	require.Eventually(t, func() bool {
		_, exists := analytic.SnapshotNodeMap()[node.ID]
		return !exists
	}, time.Second, 10*time.Millisecond, "deleted node remained in the analytic snapshot")
}

func TestDeleteNodePermanentlyDeletesATrashedNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNodeLifecycleTest(t)

	node := &model.Node{
		Name:       "trashed-node",
		URL:        "https://node.example",
		AuthMethod: model.NodeAuthMethodPaired,
		Enabled:    true,
	}
	require.NoError(t, db.Create(node).Error)
	credential := &model.NodeCredential{
		NodeID:              node.ID,
		CredentialID:        "credential-id",
		TargetInstanceID:    "target-instance-id",
		PublicKey:           []byte("public-key"),
		EncryptedPrivateKey: []byte("private-key"),
		Status:              model.NodeCredentialStatusActive,
	}
	require.NoError(t, db.Create(credential).Error)

	router := gin.New()
	router.DELETE("/nodes/:id", DeleteNode)
	nodePath := "/nodes/" + strconv.FormatUint(node.ID, 10)

	softDeleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(softDeleteRecorder, httptest.NewRequest(http.MethodDelete, nodePath, nil))
	require.Equal(t, http.StatusNoContent, softDeleteRecorder.Code)

	var trashed model.Node
	require.NoError(t, db.Unscoped().First(&trashed, node.ID).Error)
	require.NotNil(t, trashed.DeletedAt)
	require.NoError(t, db.First(&model.NodeCredential{}, credential.ID).Error,
		"a recoverable delete must retain the node credential")

	permanentDeleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(permanentDeleteRecorder,
		httptest.NewRequest(http.MethodDelete, nodePath+"?permanent=true", nil))
	require.Equal(t, http.StatusNoContent, permanentDeleteRecorder.Code)
	require.ErrorIs(t, db.Unscoped().First(&model.Node{}, node.ID).Error, gorm.ErrRecordNotFound)
	require.ErrorIs(t, db.Unscoped().First(&model.NodeCredential{}, credential.ID).Error, gorm.ErrRecordNotFound)
}

func TestRecoverNodeRestoresNodeAndRetainsCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNodeLifecycleTest(t)

	node := &model.Node{Name: "recoverable-node", URL: "https://node.example"}
	require.NoError(t, db.Create(node).Error)
	credential := &model.NodeCredential{
		NodeID:              node.ID,
		CredentialID:        "recoverable-credential",
		TargetInstanceID:    "target-instance-id",
		PublicKey:           []byte("public-key"),
		EncryptedPrivateKey: []byte("private-key"),
		Status:              model.NodeCredentialStatusActive,
	}
	require.NoError(t, db.Create(credential).Error)
	require.NoError(t, db.Delete(node).Error)

	router := gin.New()
	router.PATCH("/nodes/:id", RecoverNode)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch,
		"/nodes/"+strconv.FormatUint(node.ID, 10), nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.NoError(t, db.First(&model.Node{}, node.ID).Error)
	require.NoError(t, db.First(&model.NodeCredential{}, credential.ID).Error)
}

func TestNodeRouterRegistersRecoveryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	InitRouter(router.Group("/api"))

	for _, route := range router.Routes() {
		if route.Method == http.MethodPatch && route.Path == "/api/nodes/:id" {
			return
		}
	}
	t.Fatal("PATCH /api/nodes/:id recovery route is not registered")
}

func TestNodeRouterRegistersAuthenticationUpgradeRetryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	InitRouter(router.Group("/api"))

	for _, route := range router.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/nodes/:id/auth-upgrade/retry" {
			return
		}
	}
	t.Fatal("POST /api/nodes/:id/auth-upgrade/retry is not registered")
}
