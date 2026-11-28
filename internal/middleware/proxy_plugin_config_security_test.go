package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	backupapi "github.com/0xJacky/Nginx-UI/api/backup"
	sitesapi "github.com/0xJacky/Nginx-UI/api/sites"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
)

// GHSA-fpv9-q72v-fgrc, GHSA-8f3g-q7q2-4w6r: the form values of a plugin
// probe kind (a site's probe_config) and of a plugin storage backend (an auto
// backup's storage_config) may be credentials. A service token that lacks
// api:write must not receive the probe values, and no service token receives
// the storage values, both on this node and relayed from a child node. The
// child answers the controller's node principal in full, so the relayed
// response must match what the same caller gets locally. Interactive
// administrators keep the values the editors need to round-trip.
func TestPluginConfigBlobsAreRedactedForServiceTokensLocallyAndRelayed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init("fatal")
	cache.InitInMemoryCache()
	oldCrypto := appsettings.CryptoSettings.Secret
	oldInstance := appsettings.NodeSettings.InstanceID
	oldSecret := appsettings.NodeSettings.Secret
	oldJWT := cosysettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		appsettings.CryptoSettings.Secret = oldCrypto
		appsettings.NodeSettings.InstanceID = oldInstance
		appsettings.NodeSettings.Secret = oldSecret
		cosysettings.AppSettings.JwtSecret = oldJWT
		model.Use(nil)
	})
	appsettings.CryptoSettings.Secret = "plugin-config-redaction-crypto-root"
	appsettings.NodeSettings.InstanceID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	appsettings.NodeSettings.Secret = "plugin-config-redaction-node-secret"
	cosysettings.AppSettings.JwtSecret = "plugin-config-redaction-jwt"

	cosy.RegisterModels(&model.User{}, &model.Passkey{}, &model.AuthToken{}, &model.MCPServiceToken{}, &model.Node{},
		&model.SiteConfig{}, &model.AutoBackup{})
	db := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	admin := &model.User{Name: "plugin-config-admin", Status: true}
	require.NoError(t, db.Create(admin).Error)
	t.Cleanup(func() { internaluser.InvalidateUserCache(admin.ID) })
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)

	const probeSecret, storageSecret = "PROBE-SECRET-5", "STORAGE-SECRET-6"
	siteConfig := &model.SiteConfig{
		SiteKey: "example.com:443", SiteName: "example", Host: "example.com:443", Scheme: "https",
		ProbeKind: "plugin:redis", ProbeConfig: map[string]string{"password": probeSecret},
	}
	require.NoError(t, db.Create(siteConfig).Error)
	autoBackup := &model.AutoBackup{
		Name: "plugin-storage-task", BackupType: model.BackupTypeNginxAndNginxUI, StorageType: "plugin:store",
		StoragePath: "nginx-ui", CronExpression: "0 0 * * *",
		StorageConfig: map[string]string{"secret_key": storageSecret},
	}
	require.NoError(t, db.Create(autoBackup).Error)

	register := func(g *gin.RouterGroup) {
		sitesapi.InitRouter(g)
		backupapi.InitAutoBackupRouter(g)
	}

	// The child is a separate engine on a loopback listener; both engines
	// share the test database, so the child serves the same rows.
	child := gin.New()
	register(child.Group("/api", middleware.AuthRequired()))
	childServer := httptest.NewServer(child)
	defer childServer.Close()

	node := &model.Node{Name: "child", URL: childServer.URL, Enabled: true, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, db.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte(appsettings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", encrypted).Error)

	controller := gin.New()
	register(controller.Group("/api", middleware.AuthRequired(), middleware.Proxy()))

	id := func(value uint64) string { return strconv.FormatUint(value, 10) }
	get := func(t *testing.T, path, token string, remote bool) string {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
		req.Header.Set("Authorization", "Bearer "+token)
		if remote {
			req.Header.Set("X-Node-ID", id(node.ID))
		}
		res := httptest.NewRecorder()
		controller.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		return res.Body.String()
	}

	type caller struct {
		name  string
		token string
	}
	reader := caller{"api:read", readToken}
	writer := caller{"api:write", writeToken}
	administrator := caller{"interactive administrator", payload.Token}

	cases := []struct {
		path    string
		secret  string
		keep    string
		hidden  []caller
		visible []caller
	}{
		{
			path: "/api/site_navigation/health_check/" + id(siteConfig.ID), secret: probeSecret, keep: `"password"`,
			hidden: []caller{reader}, visible: []caller{writer, administrator},
		},
		{
			path: "/api/auto_backup/" + id(autoBackup.ID), secret: storageSecret, keep: autoBackup.Name,
			hidden: []caller{reader, writer}, visible: []caller{administrator},
		},
		{
			path: "/api/auto_backup", secret: storageSecret, keep: autoBackup.Name,
			hidden: []caller{reader, writer}, visible: []caller{administrator},
		},
	}
	for _, tc := range cases {
		check := func(c caller, seesSecret bool) {
			t.Run(c.name+" "+tc.path, func(t *testing.T) {
				local := get(t, tc.path, c.token, false)
				relayed := get(t, tc.path, c.token, true)
				require.Contains(t, local, tc.keep, "the rest of the record is still returned")
				require.Equal(t, seesSecret, strings.Contains(local, tc.secret), local)
				// The controller's relay redaction must leave exactly what the
				// local handler returns to the same caller, no more and no less.
				require.JSONEq(t, local, relayed)
			})
		}
		for _, c := range tc.hidden {
			check(c, false)
		}
		for _, c := range tc.visible {
			check(c, true)
		}
	}
}
