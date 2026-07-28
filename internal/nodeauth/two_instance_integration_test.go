package nodeauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestTwoInstanceOperationalRequestsUsePairedAuthentication(t *testing.T) {
	const (
		parentInstanceID = "11111111-1111-4111-8111-111111111111"
		childInstanceID  = "22222222-2222-4222-8222-222222222222"
		credentialID     = "33333333-3333-4333-8333-333333333333"
	)

	originalInstanceID := settings.NodeSettings.InstanceID
	originalCryptoSecret := settings.CryptoSettings.Secret
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		settings.CryptoSettings.Secret = originalCryptoSecret
		model.Use(nil)
	})
	settings.NodeSettings.InstanceID = parentInstanceID
	settings.CryptoSettings.Secret = "two-instance-integration-root"

	parentDatabase := openNodeAuthIntegrationDatabase(t, "parent")
	require.NoError(t, parentDatabase.AutoMigrate(&model.Node{}, &model.NodeCredential{}))
	childDatabase := openNodeAuthIntegrationDatabase(t, "child")
	require.NoError(t, childDatabase.AutoMigrate(&model.NodeControllerCredential{}))

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encryptedPrivateKey, err := EncryptPrivateCredential(SigningCredentialPurpose(credentialID), privateKey)
	require.NoError(t, err)

	node := &model.Node{
		Name:             "child",
		URL:              "https://child.example",
		AuthMethod:       model.NodeAuthMethodPaired,
		CredentialStatus: model.NodeCredentialStatusActive,
		Enabled:          true,
	}
	require.NoError(t, parentDatabase.Create(node).Error)
	require.NoError(t, parentDatabase.Create(&model.NodeCredential{
		NodeID:              node.ID,
		CredentialID:        credentialID,
		TargetInstanceID:    childInstanceID,
		PublicKey:           publicKey,
		EncryptedPrivateKey: encryptedPrivateKey,
		Status:              model.NodeCredentialStatusActive,
	}).Error)
	require.NoError(t, childDatabase.Create(&model.NodeControllerCredential{
		CredentialID:         credentialID,
		ControllerInstanceID: parentInstanceID,
		PublicKey:            publicKey,
		Status:               model.NodeCredentialStatusActive,
	}).Error)
	model.Use(parentDatabase)

	replayCache := NewReplayCache(100)
	verifiedPaths := make(map[string]int)
	childVerifier := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		settings.NodeSettings.InstanceID = childInstanceID
		principal, verifyErr := verifyRequest(request, childDatabase, time.Now(), replayCache)
		settings.NodeSettings.InstanceID = parentInstanceID
		defer CloseStagedBody(request)
		if verifyErr != nil {
			return nil, verifyErr
		}
		if principal.ControllerInstanceID != parentInstanceID || principal.AuthMethod != model.NodeAuthMethodPaired {
			return nil, fmt.Errorf("unexpected verified node principal")
		}
		if request.Header.Get("X-Node-Secret") != "" {
			return nil, fmt.Errorf("paired request retained a legacy secret")
		}
		verifiedPaths[request.URL.Path]++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     http.StatusText(http.StatusNoContent),
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    request,
		}, nil
	})

	client := &http.Client{Transport: NewTransport(node, childVerifier)}
	operations := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "monitoring", method: http.MethodGet, url: "https://child.example/api/node"},
		{name: "HTTP proxy", method: http.MethodGet, url: "https://child.example/api/configs?sort=name"},
		{name: "configuration", method: http.MethodPost, url: "https://child.example/api/configs", body: `{"name":"example"}`},
		{name: "sites", method: http.MethodPost, url: "https://child.example/api/sites/example.com", body: `{"content":"server {}"}`},
		{name: "streams", method: http.MethodPost, url: "https://child.example/api/streams/tcp", body: `{"content":"server {}"}`},
		{name: "certificates", method: http.MethodPut, url: "https://child.example/api/cert_sync", body: `{"certificate":"payload"}`},
		{name: "restart", method: http.MethodPost, url: "https://child.example/api/nginx/restart"},
		{name: "upgrade", method: http.MethodGet, url: "https://child.example/api/upgrade/perform"},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			var body io.Reader
			if operation.body != "" {
				body = strings.NewReader(operation.body)
			}
			request, err := http.NewRequest(operation.method, operation.url, body)
			require.NoError(t, err)
			response, err := client.Do(request)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			assert.Equal(t, http.StatusNoContent, response.StatusCode)
		})
	}

	for _, path := range []string{
		"/api/node",
		"/api/configs",
		"/api/sites/example.com",
		"/api/streams/tcp",
		"/api/cert_sync",
		"/api/nginx/restart",
		"/api/upgrade/perform",
	} {
		assert.NotZero(t, verifiedPaths[path], "operation path was not verified: %s", path)
	}

	for _, webSocketURL := range []string{
		"wss://child.example/api/analytic/intro",
		"wss://child.example/api/pty?rows=24&cols=80",
	} {
		headers := make(http.Header)
		require.NoError(t, SignWebSocketHeaders(node, webSocketURL, headers))
		request, err := http.NewRequest(http.MethodGet, webSocketURL, nil)
		require.NoError(t, err)
		request.Header = headers
		settings.NodeSettings.InstanceID = childInstanceID
		principal, err := verifyRequest(request, childDatabase, time.Now(), replayCache)
		settings.NodeSettings.InstanceID = parentInstanceID
		CloseStagedBody(request)
		require.NoError(t, err)
		assert.Equal(t, parentInstanceID, principal.ControllerInstanceID)
	}

	var parentCredential model.NodeCredential
	require.NoError(t, parentDatabase.Where("credential_id = ?", credentialID).First(&parentCredential).Error)
	assert.NotNil(t, parentCredential.LastUsedAt)
	var childCredential model.NodeControllerCredential
	require.NoError(t, childDatabase.Where("credential_id = ?", credentialID).First(&childCredential).Error)
	assert.NotNil(t, childCredential.LastUsedAt)
}

func openNodeAuthIntegrationDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return database
}
