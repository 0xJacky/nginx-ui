package certificate

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

func TestCertificateExportPreservesManagementTokenScopes(t *testing.T) {
	database := setupSelfSignedAPITest(t)
	cache.InitInMemoryCache()
	previousNginx, previousNode := *settings.NginxSettings, *settings.NodeSettings
	previousCrypto, previousJWT := settings.CryptoSettings.Secret, cosysettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		cache.Shutdown()
		model.Use(nil)
		*settings.NginxSettings, *settings.NodeSettings = previousNginx, previousNode
		settings.CryptoSettings.Secret, cosysettings.AppSettings.JwtSecret = previousCrypto, previousJWT
	})
	settings.CryptoSettings.Secret = "certificate-export-crypto-root"
	cosysettings.AppSettings.JwtSecret = "certificate-export-jwt-secret"
	settings.NodeSettings.InstanceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	settings.NodeSettings.Secret = "certificate-export-child-secret"
	settings.NodeSettings.Demo = false
	root := t.TempDir()
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	settings.NginxSettings.HostMode = ""
	settings.NginxSettings.ContainerName = ""
	require.NoError(t, os.WriteFile(settings.NginxSettings.ConfigPath, []byte("events {}\nhttp {}\n"), 0o600))
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.MCPServiceToken{}, &model.Node{}))
	admin := &model.User{Name: "certificate-admin", Status: true, Language: "en"}
	require.NoError(t, database.Create(admin).Error)
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	_, reader, err := internalmcp.CreateServiceToken("certificate-reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	_, writer, err := internalmcp.CreateServiceToken("certificate-writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)
	certPEM, keyPEM := newDownloadTestPair(t)
	certPath, keyPath := filepath.Join(root, "cert.pem"), filepath.Join(root, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	certificate := &model.Cert{Name: "export-example", SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
	require.NoError(t, database.Create(certificate).Error)
	route := "/api/certs/" + strconv.FormatUint(certificate.ID, 10) + "/download"
	child := gin.New()
	InitCertificateRouter(child.Group("/api", middleware.AuthRequired()))
	childServer := httptest.NewServer(child)
	defer childServer.Close()
	node := &model.Node{Name: "export-child", URL: childServer.URL, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, database.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte(settings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, database.Model(node).Update("encrypted_legacy_secret", encrypted).Error)
	controller := gin.New()
	InitCertificateRouter(controller.Group("/api", middleware.AuthRequired(), middleware.Proxy()))
	request := func(router http.Handler, token, format string, remote bool) *httptest.ResponseRecorder {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBufferString(`{"format":"`+format+`"}`)).WithContext(ctx)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if remote {
			req.Header.Set("X-Node-ID", strconv.FormatUint(node.ID, 10))
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}
	for _, target := range []struct {
		name   string
		router http.Handler
		remote bool
	}{{"local", child, false}, {"proxied", controller, true}} {
		t.Run(target.name, func(t *testing.T) {
			for _, token := range []string{writer, payload.Token} {
				for _, format := range []string{"crt", "key", "pfx", "zip"} {
					recorder := request(target.router, token, format, target.remote)
					require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					require.NotEmpty(t, recorder.Body.Bytes())
					switch format {
					case "crt":
						require.Equal(t, certPEM, recorder.Body.Bytes())
					case "key":
						require.Equal(t, keyPEM, recorder.Body.Bytes())
					case "zip":
						files := readTestZip(t, recorder.Body.Bytes())
						require.Equal(t, keyPEM, files["export-example.key"])
					}
				}
			}
			for _, token := range []string{reader, "invalid-token"} {
				recorder := request(target.router, token, "key", target.remote)
				require.Equal(t, http.StatusForbidden, recorder.Code)
				require.NotContains(t, recorder.Body.String(), "BEGIN PRIVATE KEY")
			}
		})
	}
}
