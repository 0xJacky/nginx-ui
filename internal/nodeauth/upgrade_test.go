package nodeauth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	testLegacySecret         = "shared-legacy-node-secret"
	testControllerInstanceID = "22222222-2222-4222-8222-222222222222"
	testTargetInstanceID     = "11111111-1111-4111-8111-111111111111"
)

func TestUpgradeConfirmationAuthenticatesTheTarget(t *testing.T) {
	const credentialID = "44444444-4444-4444-8444-444444444444"
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	confirmation, err := SignUpgradeConfirmation([]byte(testLegacySecret), testControllerInstanceID,
		publicKey, credentialID, testTargetInstanceID)
	require.NoError(t, err)

	assert.NoError(t, VerifyUpgradeConfirmation([]byte(testLegacySecret), testControllerInstanceID,
		publicKey, credentialID, testTargetInstanceID, confirmation))

	t.Run("other_secret", func(t *testing.T) {
		assert.ErrorIs(t, VerifyUpgradeConfirmation([]byte("another-secret"), testControllerInstanceID,
			publicKey, credentialID, testTargetInstanceID, confirmation), ErrUpgradeProofInvalid)
	})

	t.Run("other_controller", func(t *testing.T) {
		assert.ErrorIs(t, VerifyUpgradeConfirmation([]byte(testLegacySecret),
			"33333333-3333-4333-8333-333333333333",
			publicKey, credentialID, testTargetInstanceID, confirmation), ErrUpgradeProofInvalid)
	})

	t.Run("substituted_public_key", func(t *testing.T) {
		substituted, _, generateErr := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, generateErr)
		assert.ErrorIs(t, VerifyUpgradeConfirmation([]byte(testLegacySecret), testControllerInstanceID,
			substituted, credentialID, testTargetInstanceID, confirmation), ErrUpgradeProofInvalid)
	})

	t.Run("other_credential", func(t *testing.T) {
		assert.ErrorIs(t, VerifyUpgradeConfirmation([]byte(testLegacySecret), testControllerInstanceID,
			publicKey, "55555555-5555-4555-8555-555555555555", testTargetInstanceID,
			confirmation), ErrUpgradeProofInvalid)
	})
}

func setupUpgradeControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalInstanceID := settings.NodeSettings.InstanceID
	originalCryptoSecret := settings.CryptoSettings.Secret
	originalNodeSecret := settings.NodeSettings.Secret
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		settings.CryptoSettings.Secret = originalCryptoSecret
		settings.NodeSettings.Secret = originalNodeSecret
		model.Use(nil)
	})
	settings.NodeSettings.InstanceID = testControllerInstanceID
	settings.CryptoSettings.Secret = "legacy-upgrade-root-key"
	settings.NodeSettings.Secret = testLegacySecret

	database := openNodeAuthIntegrationDatabase(t, "controller")
	require.NoError(t, database.AutoMigrate(&model.Node{}, &model.NodeCredential{}))
	model.Use(database)
	return database
}

func createLegacyNode(t *testing.T, database *gorm.DB, name, url string) *model.Node {
	t.Helper()
	encryptedSecret, err := EncryptPrivateCredential(LegacyCredentialPurpose(1), []byte(testLegacySecret))
	require.NoError(t, err)
	node := &model.Node{
		Name:                  name,
		URL:                   url,
		AuthMethod:            model.NodeAuthMethodLegacy,
		CredentialStatus:      model.NodeCredentialStatusActive,
		EncryptedLegacySecret: encryptedSecret,
		Enabled:               true,
	}
	require.NoError(t, database.Create(node).Error)
	require.EqualValues(t, 1, node.ID, "the legacy credential purpose is bound to the node ID")
	return node
}

// TestLegacyUpgradeUsesPreV250CompatibleAuthentication exercises the rolling
// upgrade path: the first request uses the old header protocol, and a successful
// handshake immediately replaces it with a dedicated key pair.
func TestLegacyUpgradeUsesPreV250CompatibleAuthentication(t *testing.T) {
	database := setupUpgradeControllerTest(t)

	credentialID := uuid.NewString()
	var (
		observedHeaders http.Header
		observedBody    []byte
	)
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/node/pair/upgrade", request.URL.Path)
		observedHeaders = request.Header.Clone()

		// Stand in for a target that still uses the pre-v2.5.0 node protocol.
		if request.Header.Get("X-Node-Secret") != testLegacySecret {
			writer.WriteHeader(http.StatusForbidden)
			return
		}

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		CloseStagedBody(request)
		observedBody = body

		var payload upgradeRequest
		require.NoError(t, json.Unmarshal(body, &payload))
		publicKey, err := base64.RawURLEncoding.DecodeString(payload.PublicKey)
		require.NoError(t, err)

		confirmation, err := SignUpgradeConfirmation([]byte(testLegacySecret),
			payload.ControllerInstanceID, publicKey, credentialID, testTargetInstanceID)
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(pairingResponse{
			CredentialID:     credentialID,
			TargetInstanceID: testTargetInstanceID,
			Confirmation:     confirmation,
		}))
	}))
	t.Cleanup(target.Close)

	node := createLegacyNode(t, database, "child", target.URL)

	result, err := UpgradeLegacyRelationship(context.Background(), node, testControllerInstanceID)
	require.NoError(t, err)
	assert.Equal(t, credentialID, result.CredentialID)

	assert.Equal(t, testLegacySecret, observedHeaders.Get("X-Node-Secret"))
	assert.Empty(t, observedHeaders.Get(signatureHeader))
	assert.NotContains(t, string(observedBody), testLegacySecret, "the upgrade must not carry the shared secret")

	var stored model.Node
	require.NoError(t, database.First(&stored, node.ID).Error)
	assert.Equal(t, model.NodeAuthMethodPaired, stored.AuthMethod)
	assert.Equal(t, model.NodeCredentialStatusActive, stored.CredentialStatus)
	assert.Equal(t, model.NodeAuthUpgradeStatusCompleted, stored.AuthUpgradeStatus)
	assert.Equal(t, model.NodeAuthUpgradeStepCompleted, stored.AuthUpgradeStep)
	assert.NotNil(t, stored.AuthUpgradeCompletedAt)
	assert.Empty(t, stored.Token)
	assert.NotEmpty(t, stored.EncryptedLegacySecret, "the retained secret is what makes recovery automatic")

	var credential model.NodeCredential
	require.NoError(t, database.Where("node_id = ?", node.ID).First(&credential).Error)
	assert.Equal(t, credentialID, credential.CredentialID)
	assert.Equal(t, testTargetInstanceID, credential.TargetInstanceID)
	assert.Equal(t, model.NodeCredentialStatusActive, credential.Status)
}

// TestLegacyUpgradeLeavesOlderNodesOnTheSharedSecret covers a rolling upgrade:
// a node still running a release without the pairing endpoint keeps working
// exactly as before, and the maintenance pass does not treat it as a fault.
func TestLegacyUpgradeLeavesOlderNodesOnTheSharedSecret(t *testing.T) {
	database := setupUpgradeControllerTest(t)

	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, testLegacySecret, request.Header.Get("X-Node-Secret"))
		assert.Empty(t, request.Header.Get(signatureHeader))
		writer.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(target.Close)

	node := createLegacyNode(t, database, "outdated-child", target.URL)

	_, err := UpgradeLegacyRelationship(context.Background(), node, testControllerInstanceID)
	require.ErrorIs(t, err, ErrRelationshipUnsupported)

	assert.Empty(t, MaintainRelationships(context.Background(), testControllerInstanceID, time.Now()),
		"a node awaiting its own upgrade is not a maintenance issue")

	var stored model.Node
	require.NoError(t, database.First(&stored, node.ID).Error)
	assert.Equal(t, model.NodeAuthMethodLegacy, stored.AuthMethod)
	assert.NotEmpty(t, stored.EncryptedLegacySecret)
	assert.Equal(t, model.NodeAuthUpgradeStatusWaitingTarget, stored.AuthUpgradeStatus)
	assert.Equal(t, model.NodeAuthUpgradeErrorTargetUnsupported, stored.AuthUpgradeErrorCode)
	assert.Equal(t, model.NodeAuthUpgradeStepRequest, stored.AuthUpgradeStep)
	assert.EqualValues(t, 1, stored.AuthUpgradeAttemptCount)
	assert.NotNil(t, stored.AuthUpgradeNextRetryAt)
}

func TestLegacyUpgradeRejectsUnconfirmedTarget(t *testing.T) {
	database := setupUpgradeControllerTest(t)

	// An attacker able to rewrite the response still cannot produce a
	// confirmation, so the controller must refuse the credential it was handed.
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(pairingResponse{
			CredentialID:     uuid.NewString(),
			TargetInstanceID: testTargetInstanceID,
			Confirmation:     strings.Repeat("A", 43),
		}))
	}))
	t.Cleanup(target.Close)

	node := createLegacyNode(t, database, "child", target.URL)

	_, err := UpgradeLegacyRelationship(context.Background(), node, testControllerInstanceID)
	require.ErrorIs(t, err, ErrUpgradeProofInvalid)

	var stored model.Node
	require.NoError(t, database.First(&stored, node.ID).Error)
	assert.Equal(t, model.NodeAuthMethodLegacy, stored.AuthMethod, "a rejected upgrade must not change the node")
	var count int64
	require.NoError(t, database.Model(&model.NodeCredential{}).Where("node_id = ?", node.ID).Count(&count).Error)
	assert.Zero(t, count)
}

func TestRunLegacyRelationshipUpgradePersistsVerificationFailure(t *testing.T) {
	database := setupUpgradeControllerTest(t)
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(pairingResponse{
			CredentialID:     uuid.NewString(),
			TargetInstanceID: testTargetInstanceID,
			Confirmation:     strings.Repeat("A", 43),
		}))
	}))
	t.Cleanup(target.Close)

	node := createLegacyNode(t, database, "invalid-confirmation", target.URL)
	now := time.Now()
	err := RunLegacyRelationshipUpgrade(context.Background(), node.ID, testControllerInstanceID, now)
	require.ErrorIs(t, err, ErrUpgradeProofInvalid)

	var stored model.Node
	require.NoError(t, database.First(&stored, node.ID).Error)
	assert.Equal(t, model.NodeAuthMethodLegacy, stored.AuthMethod)
	assert.Equal(t, model.NodeAuthUpgradeStatusFailed, stored.AuthUpgradeStatus)
	assert.Equal(t, model.NodeAuthUpgradeStepVerify, stored.AuthUpgradeStep)
	assert.Equal(t, model.NodeAuthUpgradeErrorInvalidConfirmation, stored.AuthUpgradeErrorCode)
	assert.EqualValues(t, 1, stored.AuthUpgradeAttemptCount)
	require.NotNil(t, stored.AuthUpgradeNextRetryAt)
	assert.WithinDuration(t, now.Add(relationshipUpgradeRetryDelay), *stored.AuthUpgradeNextRetryAt, time.Millisecond)

	var count int64
	require.NoError(t, database.Model(&model.NodeCredential{}).Where("node_id = ?", node.ID).Count(&count).Error)
	assert.Zero(t, count, "a rejected confirmation must not create a credential")
}

func TestRetryLegacyRelationshipUpgradeRequeuesAFailedNode(t *testing.T) {
	database := setupUpgradeControllerTest(t)
	node := createLegacyNode(t, database, "failed-node", "https://node.example")
	require.NoError(t, database.Model(node).Updates(map[string]any{
		"auth_upgrade_status":     model.NodeAuthUpgradeStatusFailed,
		"auth_upgrade_step":       model.NodeAuthUpgradeStepVerify,
		"auth_upgrade_error_code": model.NodeAuthUpgradeErrorInvalidConfirmation,
		"auth_upgrade_error":      "sanitized error",
	}).Error)

	now := time.Now()
	require.NoError(t, RetryLegacyRelationshipUpgrade(node.ID, now))

	var stored model.Node
	require.NoError(t, database.First(&stored, node.ID).Error)
	assert.Equal(t, model.NodeAuthUpgradeStatusPending, stored.AuthUpgradeStatus)
	assert.Equal(t, model.NodeAuthUpgradeStepQueued, stored.AuthUpgradeStep)
	assert.Empty(t, stored.AuthUpgradeErrorCode)
	assert.Empty(t, stored.AuthUpgradeError)
	require.NotNil(t, stored.AuthUpgradeNextRetryAt)
	assert.WithinDuration(t, now, *stored.AuthUpgradeNextRetryAt, time.Millisecond)
}
