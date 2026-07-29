package nodeauth

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const sharedNodeSecret = "shared-node-secret"

func setupSharedSecretTest(t *testing.T) *gorm.DB {
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
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	settings.CryptoSettings.Secret = "shared-secret-test-root"
	settings.NodeSettings.Secret = sharedNodeSecret

	database := openNodeAuthIntegrationDatabase(t, "shared-secret")
	require.NoError(t, database.AutoMigrate(&model.Node{}, &model.NodeControllerCredential{}))
	model.Use(database)
	return database
}

func newSharedSecretRequest(t *testing.T, secret string, now time.Time) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, "https://node.example/api/configs?sort=name",
		strings.NewReader(`{"name":"example"}`))
	require.NoError(t, err)
	require.NoError(t, SignRequestWithSharedSecret(request, []byte(secret), now))
	return request
}

// TestSharedSecretRequestKeepsTheSecretOffTheWire is the property the whole
// change exists for: a node that authenticates with the configured secret no
// longer transmits it.
func TestSharedSecretRequestKeepsTheSecretOffTheWire(t *testing.T) {
	database := setupSharedSecretTest(t)
	now := time.Now()

	request := newSharedSecretRequest(t, sharedNodeSecret, now)
	assert.Empty(t, request.Header.Get("X-Node-Secret"))
	for _, header := range []string{signatureInputHeader, signatureHeader, contentDigestHeader} {
		assert.NotEmpty(t, request.Header.Get(header), header)
	}
	body, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), sharedNodeSecret)
	assert.NotContains(t, request.URL.String(), sharedNodeSecret)
	for name, values := range request.Header {
		for _, value := range values {
			assert.NotContains(t, value, sharedNodeSecret, name)
		}
	}

	principal, err := verifyRequest(newSharedSecretRequest(t, sharedNodeSecret, now),
		database, now, NewReplayCache(16))
	require.NoError(t, err)
	assert.Equal(t, model.NodeAuthMethodLegacy, principal.AuthMethod)
}

func TestSharedSecretRequestRejectsForgeryAndReplay(t *testing.T) {
	database := setupSharedSecretTest(t)
	now := time.Now()

	t.Run("other_secret", func(t *testing.T) {
		_, err := verifyRequest(newSharedSecretRequest(t, "another-secret", now),
			database, now, NewReplayCache(16))
		assert.ErrorContains(t, err, "signature is invalid")
	})

	t.Run("tampered_path", func(t *testing.T) {
		request := newSharedSecretRequest(t, sharedNodeSecret, now)
		request.URL.Path = "/api/nginx/restart"
		_, err := verifyRequest(request, database, now, NewReplayCache(16))
		assert.ErrorContains(t, err, "signature is invalid")
	})

	t.Run("tampered_body", func(t *testing.T) {
		request := newSharedSecretRequest(t, sharedNodeSecret, now)
		CloseStagedBody(request)
		request.Body = io.NopCloser(strings.NewReader(`{"name":"attacker"}`))
		_, err := verifyRequest(request, database, now, NewReplayCache(16))
		assert.ErrorContains(t, err, "content digest mismatch")
	})

	t.Run("replay", func(t *testing.T) {
		cache := NewReplayCache(16)
		request := newSharedSecretRequest(t, sharedNodeSecret, now)
		replay := request.Clone(request.Context())
		replay.Body = io.NopCloser(strings.NewReader(`{"name":"example"}`))

		_, err := verifyRequest(request, database, now, cache)
		require.NoError(t, err)
		_, err = verifyRequest(replay, database, now, cache)
		assert.ErrorContains(t, err, "nonce was already used")
	})

	t.Run("expired", func(t *testing.T) {
		_, err := verifyRequest(newSharedSecretRequest(t, sharedNodeSecret, now),
			database, now.Add(10*time.Minute), NewReplayCache(16))
		assert.ErrorContains(t, err, "expired")
	})

	t.Run("no_configured_secret", func(t *testing.T) {
		request := newSharedSecretRequest(t, sharedNodeSecret, now)
		settings.NodeSettings.Secret = ""
		defer func() { settings.NodeSettings.Secret = sharedNodeSecret }()
		_, err := verifyRequest(request, database, now, NewReplayCache(16))
		assert.ErrorContains(t, err, "not configured")
	})
}

// TestLegacyNodeTransportSignsInsteadOfSendingTheSecret covers the controller
// side end to end: the node still authenticates with the shared secret, but the
// transport now signs every request with it.
func TestLegacyNodeTransportSignsInsteadOfSendingTheSecret(t *testing.T) {
	database := setupSharedSecretTest(t)

	encryptedSecret, err := EncryptPrivateCredential(LegacyCredentialPurpose(1), []byte(sharedNodeSecret))
	require.NoError(t, err)
	node := &model.Node{
		Name:                  "child",
		URL:                   "https://child.example",
		AuthMethod:            model.NodeAuthMethodLegacy,
		CredentialStatus:      model.NodeCredentialStatusActive,
		EncryptedLegacySecret: encryptedSecret,
		Enabled:               true,
	}
	require.NoError(t, database.Create(node).Error)
	require.EqualValues(t, 1, node.ID, "the legacy credential purpose is bound to the node ID")

	replayCache := NewReplayCache(16)
	verified := 0
	child := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("X-Node-Secret") != "" {
			t.Error("the shared secret must not travel with the request")
		}
		principal, verifyErr := verifyRequest(request, database, time.Now(), replayCache)
		defer CloseStagedBody(request)
		if verifyErr != nil {
			return nil, verifyErr
		}
		assert.Equal(t, model.NodeAuthMethodLegacy, principal.AuthMethod)
		verified++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    request,
		}, nil
	})

	client := &http.Client{Transport: NewTransport(node, child)}
	for _, target := range []string{
		"https://child.example/api/node",
		"https://child.example/api/configs?sort=name",
	} {
		request, err := http.NewRequest(http.MethodGet, target, nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		assert.Equal(t, http.StatusNoContent, response.StatusCode)
	}
	assert.Equal(t, 2, verified)

	headers := make(http.Header)
	require.NoError(t, SignWebSocketHeaders(node, "wss://child.example/api/analytic/intro", headers))
	assert.Empty(t, headers.Get("X-Node-Secret"))
	assert.NotEmpty(t, headers.Get(signatureHeader))
}
