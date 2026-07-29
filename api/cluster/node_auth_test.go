package cluster

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	nodeSecret           = "shared-legacy-node-secret"
	controllerInstanceID = "22222222-2222-4222-8222-222222222222"
)

func TestLegacyAuthenticatedRelationshipUpgradeReplacesExistingControllerCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.NodeControllerCredential{}))
	model.Use(database)
	originalInstanceID := settings.NodeSettings.InstanceID
	originalSecret := settings.NodeSettings.Secret
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	settings.NodeSettings.Secret = nodeSecret
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		settings.NodeSettings.Secret = originalSecret
		model.Use(nil)
	})

	upgrade := func() (upgradeLegacyPairingResponse, ed25519.PublicKey) {
		publicKey, _, generateErr := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, generateErr)
		payload, marshalErr := json.Marshal(upgradeLegacyPairingRequest{
			ControllerInstanceID: controllerInstanceID,
			PublicKey:            base64.RawURLEncoding.EncodeToString(publicKey),
		})
		require.NoError(t, marshalErr)

		recorder := serveUpgrade(t, payload, legacyNodePrincipal)
		require.Equal(t, http.StatusCreated, recorder.Code)
		var response upgradeLegacyPairingResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, settings.NodeSettings.InstanceID, response.TargetInstanceID)
		return response, publicKey
	}

	first, firstKey := upgrade()
	second, _ := upgrade()
	assert.NotEqual(t, first.CredentialID, second.CredentialID)

	// The response proves the target holds the same secret, so the controller
	// can trust the credential before it starts relying on it.
	assert.NoError(t, nodeauth.VerifyUpgradeConfirmation([]byte(nodeSecret), controllerInstanceID,
		firstKey, first.CredentialID, first.TargetInstanceID, first.Confirmation))
	assert.Error(t, nodeauth.VerifyUpgradeConfirmation([]byte("another-secret"), controllerInstanceID,
		firstKey, first.CredentialID, first.TargetInstanceID, first.Confirmation))

	var credentials []model.NodeControllerCredential
	require.NoError(t, database.Unscoped().Order("created_at ASC").Find(&credentials).Error)
	require.Len(t, credentials, 2)
	assert.Equal(t, model.NodeCredentialStatusRevoked, credentials[0].Status)
	assert.NotNil(t, credentials[0].RevokedAt)
	assert.Equal(t, model.NodeCredentialStatusActive, credentials[1].Status)
	assert.Nil(t, credentials[1].RevokedAt)
}

func TestLegacyUpgradeRequiresSharedSecretAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.NodeControllerCredential{}))
	model.Use(database)
	originalInstanceID := settings.NodeSettings.InstanceID
	originalSecret := settings.NodeSettings.Secret
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	settings.NodeSettings.Secret = nodeSecret
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		settings.NodeSettings.Secret = originalSecret
		model.Use(nil)
	})

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	payload, err := json.Marshal(upgradeLegacyPairingRequest{
		ControllerInstanceID: controllerInstanceID,
		PublicKey:            base64.RawURLEncoding.EncodeToString(publicKey),
	})
	require.NoError(t, err)

	t.Run("unauthenticated", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, serveUpgrade(t, payload, nil).Code)
	})

	t.Run("paired_principal", func(t *testing.T) {
		// A controller that already holds its own key pair has nothing to
		// upgrade, and must not be able to mint a replacement credential.
		assert.Equal(t, http.StatusForbidden, serveUpgrade(t, payload, func(c *gin.Context) {
			c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{AuthMethod: model.NodeAuthMethodPaired})
		}).Code)
	})

	t.Run("no_configured_secret", func(t *testing.T) {
		settings.NodeSettings.Secret = ""
		defer func() { settings.NodeSettings.Secret = nodeSecret }()
		assert.Equal(t, http.StatusForbidden, serveUpgrade(t, payload, legacyNodePrincipal).Code)
	})

	var count int64
	require.NoError(t, database.Unscoped().Model(&model.NodeControllerCredential{}).Count(&count).Error)
	assert.Zero(t, count, "a rejected upgrade must not issue a credential")
}

// legacyNodePrincipal stands in for the middleware, which authenticates a
// shared-secret controller by verifying the signature on its request.
func legacyNodePrincipal(c *gin.Context) {
	c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{AuthMethod: model.NodeAuthMethodLegacy})
}

func serveUpgrade(t *testing.T, payload []byte, authenticate gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	if authenticate != nil {
		router.Use(authenticate)
	}
	router.POST("/pair/upgrade", UpgradeLegacyPairing)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/pair/upgrade", bytes.NewReader(payload)))
	return recorder
}
