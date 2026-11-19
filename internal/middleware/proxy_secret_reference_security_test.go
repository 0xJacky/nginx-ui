package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	configapi "github.com/0xJacky/Nginx-UI/api/config"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// A file is secret on a child node because the child's own configuration
// names it in a secret directive. The controller cannot see that
// configuration, so a read-only service token selecting the child must be
// refused the file, and get its history redacted, by the child's own rule.
// (GHSA-x49v-g3gc-3w78)
func TestAPIReadServiceTokenCannotReadReferencedSecretFileThroughProxy(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	for _, file := range referencedSecretFiles {
		t.Run(file.name, func(t *testing.T) {
			recorder := storedSecretRequest(state.controller, configPath(file.path), state.readToken, state.nodeID)
			require.NotContains(t, recorder.Body.String(), file.secret)
			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			body := decodeJSONBody(t, recorder)
			require.Equal(t, "config", body["scope"])
			require.EqualValues(t, 40019, body["code"])
		})
	}

	t.Run("configuration history", func(t *testing.T) {
		recorder := storedSecretRequest(state.controller, "/api/config_histories", state.readToken, state.nodeID)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.NotContains(t, recorder.Body.String(), controlTicketKey)
		require.NotContains(t, recorder.Body.String(), hashTicketKey)
		body := decodeJSONBody(t, recorder)
		require.Equal(t, appsettings.RedactedSensitiveValue, configBackupContent(t, body, "ticket-control.bin"))
		require.Equal(t, appsettings.RedactedSensitiveValue, configBackupContent(t, body, "ticket#1.bin"))
	})
}

// Through the proxy, a read-only service token keeps reading every file that
// is not a secret, and its history, from an up-to-date child.
func TestAPIReadServiceTokenKeepsReadingOrdinaryFilesThroughProxy(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	for _, file := range []struct{ path, content string }{
		{"nginx.conf", "ssl_session_ticket_key"},
		{"ssl/example.crt", state.certPEM},
		{"ssl/retired.txt", retiredNote},
		{"conf.d/notes#1.conf", "# notes\n"},
	} {
		t.Run(file.path, func(t *testing.T) {
			recorder := storedSecretRequest(state.controller, configPath(file.path), state.readToken, state.nodeID)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Contains(t, decodeJSONBody(t, recorder)["content"], file.content)
		})
	}

	t.Run("configuration history", func(t *testing.T) {
		recorder := storedSecretRequest(state.controller, "/api/config_histories", state.readToken, state.nodeID)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		body := decodeJSONBody(t, recorder)
		require.Equal(t, "events {}\n# previous revision\n", configBackupContent(t, body, "nginx.conf"))
	})
}

// api:write tokens and administrators keep reading every referenced secret
// file and its history through the proxy.
func TestReferencedSecretFileThroughProxyKeepsManagementCallers(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	for _, caller := range []struct{ name, token string }{
		{"api:write token", state.writeToken},
		{"interactive administrator", state.adminToken},
	} {
		for _, file := range referencedSecretFiles {
			t.Run(caller.name+"/"+file.name, func(t *testing.T) {
				recorder := storedSecretRequest(state.controller, configPath(file.path), caller.token, state.nodeID)
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, file.secret, decodeJSONBody(t, recorder)["content"])
			})
		}

		t.Run(caller.name+"/configuration history", func(t *testing.T) {
			recorder := storedSecretRequest(state.controller, "/api/config_histories", caller.token, state.nodeID)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			body := decodeJSONBody(t, recorder)
			require.Equal(t, hashTicketKey, configBackupContent(t, body, "ticket#1.bin"))
			require.Equal(t, controlTicketKey, configBackupContent(t, body, "ticket-control.bin"))
		})
	}
}

// A controller with this protection may relay to a child that predates it.
// That child ignores the request to classify its files and does not confirm,
// so the controller withholds every file from a read-only caller instead of
// relaying a secret, while management callers keep full access.
func TestAPIReadServiceTokenCannotReadSecretFileThroughOlderChild(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)
	olderNodeID := startOlderConfigChild(t)

	for _, file := range append(referencedSecretFiles[:1:1], struct{ name, path, secret string }{
		"ordinary configuration", "nginx.conf", "ssl_session_ticket_key",
	}) {
		t.Run("read-only/"+file.name, func(t *testing.T) {
			recorder := storedSecretRequest(state.controller, configPath(file.path), state.readToken, olderNodeID)
			require.NotContains(t, recorder.Body.String(), file.secret)
			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			body := decodeJSONBody(t, recorder)
			require.Equal(t, "middleware", body["scope"])
			require.EqualValues(t, 40304, body["code"])
		})
	}

	t.Run("read-only/configuration history", func(t *testing.T) {
		recorder := storedSecretRequest(state.controller, "/api/config_histories", state.readToken, olderNodeID)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.NotContains(t, recorder.Body.String(), controlTicketKey)
		body := decodeJSONBody(t, recorder)
		require.Equal(t, appsettings.RedactedSensitiveValue, configBackupContent(t, body, "ticket-control.bin"))
		require.Equal(t, appsettings.RedactedSensitiveValue, configBackupContent(t, body, "nginx.conf"))
	})

	for _, caller := range []struct{ name, token string }{
		{"api:write token", state.writeToken},
		{"interactive administrator", state.adminToken},
	} {
		t.Run(caller.name, func(t *testing.T) {
			recorder := storedSecretRequest(state.controller, configPath("ssl/ticket-control.bin"), caller.token, olderNodeID)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, controlTicketKey, decodeJSONBody(t, recorder)["content"])
		})
	}
}

// startOlderConfigChild serves the configuration API as a child node that
// never sees the controller's read-only marker, which is how a child without
// this protection behaves. It returns the node id the controller relays to.
func startOlderConfigChild(t *testing.T) uint64 {
	t.Helper()
	dropMarker := func(c *gin.Context) {
		values := c.Request.URL.Query()
		values.Del(middleware.ProxyReadOnlyServiceTokenQuery)
		c.Request.URL.RawQuery = values.Encode()
	}
	older := gin.New()
	configapi.InitRouter(older.Group("/api", dropMarker, middleware.AuthRequired()))
	server := httptest.NewServer(older)
	t.Cleanup(server.Close)

	node := &model.Node{Name: "older-child", URL: server.URL, Enabled: true, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, model.UseDB().Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(
		nodeauth.LegacyCredentialPurpose(node.ID),
		[]byte(appsettings.NodeSettings.Secret),
	)
	require.NoError(t, err)
	require.NoError(t, model.UseDB().Model(node).Update("encrypted_legacy_secret", encrypted).Error)
	return node.ID
}
