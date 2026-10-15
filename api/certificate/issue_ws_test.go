package certificate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issueOverWebSocket runs the IssueCert handler with issueCertificate
// replaced by issue, sends payload and returns the terminal response.
func issueOverWebSocket(t *testing.T, payload cert.ConfigPayload,
	issue func(ctx context.Context, name string, payload *cert.ConfigPayload, log *cert.Logger) (*model.Cert, error)) IssueCertResponse {
	t.Helper()

	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = t.TempDir()
	originalIssue := issueCertificate
	issueCertificate = issue
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		issueCertificate = originalIssue
	})
	stubDiagnoseForHint(t, func(context.Context, []string, acmehint.Options) []acmehint.Diagnostic { return nil })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/certificate/issue/:name", IssueCert)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	header := http.Header{"Origin": []string{server.URL}}
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/certificate/issue/example.com", header)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	require.NoError(t, ws.WriteJSON(payload))
	var resp IssueCertResponse
	require.NoError(t, ws.ReadJSON(&resp))
	return resp
}

func TestIssueCertUsesSharedIssuanceEntryPoint(t *testing.T) {
	var gotName string
	var gotPayload *cert.ConfigPayload
	resp := issueOverWebSocket(t, cert.ConfigPayload{ServerName: []string{"example.com"}, KeyType: "P256"},
		func(_ context.Context, name string, payload *cert.ConfigPayload, log *cert.Logger) (*model.Cert, error) {
			assert.NotNil(t, log)
			gotName, gotPayload = name, payload
			payload.KeyType = payload.GetKeyType()
			return &model.Cert{Model: model.Model{ID: 1}}, nil
		})

	assert.Equal(t, Success, resp.Status)
	assert.Equal(t, "example.com", gotName)
	require.NotNil(t, gotPayload)
	assert.Equal(t, "example.com", gotPayload.ConfigName, "the websocket keeps the pre-issuance route check")
	assert.Equal(t, certcrypto.EC256, resp.KeyType)
	assert.Equal(t, gotPayload.GetCertificatePath(), resp.SSLCertificate)
	assert.Equal(t, gotPayload.GetCertificateKeyPath(), resp.SSLCertificateKey)
}

func TestIssueCertReportsRejectedRequestWithoutHint(t *testing.T) {
	resp := issueOverWebSocket(t, cert.ConfigPayload{ServerName: []string{"example.com"}},
		func(context.Context, string, *cert.ConfigPayload, *cert.Logger) (*model.Cert, error) {
			return nil, errors.New("invalid identifier")
		})

	assert.Equal(t, Error, resp.Status)
	assert.Equal(t, "invalid identifier", resp.Message)
	assert.Nil(t, resp.Hint)
}

func TestIssueCertReportsIssuanceFailureWithHint(t *testing.T) {
	resp := issueOverWebSocket(t, cert.ConfigPayload{ServerName: []string{"example.com"}},
		func(context.Context, string, *cert.ConfigPayload, *cert.Logger) (*model.Cert, error) {
			return &model.Cert{Model: model.Model{ID: 1}}, errors.New(pebbleWrongServerErr)
		})

	assert.Equal(t, Error, resp.Status)
	assert.Contains(t, resp.Message, "returned 404")
	require.NotNil(t, resp.Hint)
	assert.Equal(t, acmehint.CodeChallengeNotServed, resp.Hint.Code)
}
