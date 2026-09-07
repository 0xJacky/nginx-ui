package nodeauth

import (
	"crypto/ed25519"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func TestPairedAuthenticationBehindPathPrefix(t *testing.T) {
	for i, prefix := range []string{"", "/", "/nui", "/nested/nui/"} {
		t.Run(fmt.Sprintf("prefix_%d", i), func(t *testing.T) {
			database, privateKey, _ := setupSignatureTest(t)
			require.NoError(t, database.AutoMigrate(&model.Node{}, &model.NodeCredential{}))
			credentialID := "22222222-2222-4222-8222-222222222222"
			encrypted, err := EncryptPrivateCredential(SigningCredentialPurpose(credentialID), privateKey)
			require.NoError(t, err)
			node := &model.Node{AuthMethod: model.NodeAuthMethodPaired, Enabled: true}
			require.NoError(t, database.Create(node).Error)
			require.NoError(t, database.Create(&model.NodeCredential{
				PublicKey: privateKey.Public().(ed25519.PublicKey), NodeID: node.ID, CredentialID: credentialID, TargetInstanceID: settings.NodeSettings.InstanceID,
				EncryptedPrivateKey: encrypted, Status: model.NodeCredentialStatusActive,
			}).Error)
			replay := NewReplayCache(100)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer CloseStagedBody(r)
				if _, err := verifyRequest(r, database, time.Now(), replay); err != nil {
					http.Error(w, err.Error(), http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
			// StripPrefix models the proxy_pass trailing-slash path rewrite.
			decodedPrefix := strings.TrimRight(prefix, "/")
			server := httptest.NewServer(http.StripPrefix(decodedPrefix, handler))
			defer server.Close()
			node.URL = server.URL + prefix
			require.NoError(t, database.Save(node).Error)
			for _, socket := range []bool{false, true} {
				for _, suffix := range []string{"/api/node", "/api/sites/a%2Fb?reload=true&source=cluster"} {
					wireURL := server.URL + strings.TrimRight(prefix, "/") + suffix
					request, err := http.NewRequest(http.MethodGet, wireURL, nil)
					require.NoError(t, err)
					client := &http.Client{Transport: http.DefaultTransport}
					if socket {
						require.NoError(t, SignWebSocketHeaders(node, strings.Replace(wireURL, "http://", "ws://", 1), request.Header))
					} else {
						client.Transport = NewTransport(node, http.DefaultTransport)
					}
					response, err := client.Do(request)
					require.NoError(t, err)
					body, err := io.ReadAll(response.Body)
					require.NoError(t, err)
					require.NoError(t, response.Body.Close())
					require.Equal(t, http.StatusNoContent, response.StatusCode, "socket=%v: %s", socket, body)
					require.Equal(t, wireURL, request.URL.String(), "signing must preserve the wire URL")
				}
			}
			for _, tamper := range []string{"path", "query"} {
				request, err := http.NewRequest(http.MethodGet, server.URL+strings.TrimRight(prefix, "/")+"/api/node?reload=true", nil)
				require.NoError(t, err)
				require.NoError(t, SignWebSocketHeaders(node, request.URL.String(), request.Header))
				if tamper == "path" {
					request.URL.Path += "/changed"
					request.URL.RawPath = ""
				} else {
					request.URL.RawQuery = "reload=false"
				}
				response, err := server.Client().Do(request)
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
				require.Equal(t, http.StatusForbidden, response.StatusCode)
			}
		})
	}
}

func TestNodeSigningURLPreservesPathBinding(t *testing.T) {
	for _, test := range []struct {
		name, base, wire, want string
		invalid                bool
	}{
		{"no prefix", "https://node", "https://node/api/node?q=a%2Fb", "/api/node", false},
		{"nested prefix", "https://node/nui/child/", "https://node/nui/child/api/nui", "/api/nui", false},
		{"encoded path", "https://node/n%75i", "https://node/n%75i/api/a%2Fb", "/api/a%2Fb", false},
		{"segment boundary", "https://node/nui", "https://node/nuix/api/node", "", true},
		{"missing prefix", "https://node/nui", "https://node/api/node", "", true},
		{"invalid base", "://bad", "https://node/api/node", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire, err := url.Parse(test.wire)
			require.NoError(t, err)
			signed, err := nodeSigningURL(wire, test.base)
			if test.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, signed.EscapedPath())
			require.Equal(t, wire.RawQuery, signed.RawQuery)
			require.Equal(t, test.wire, wire.String())
		})
	}
}
