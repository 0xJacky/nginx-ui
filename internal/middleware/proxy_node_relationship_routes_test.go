package middleware_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	relationshipChildInstanceID      = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	relationshipControllerInstanceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	relationshipCredentialID         = "11111111-1111-4111-8111-111111111111"
)

// relationshipFixture is a controller and a loopback child that mount the
// routers of router.InitRouter. Both share one database: the controller keeps
// its node rows and signing keys in it, the child the controller credentials
// it accepts.
type relationshipFixture struct {
	db                *gorm.DB
	controller        *gin.Engine
	childURL          string
	childNodeRequests atomic.Int32
	legacyNodeID      uint64
	pairedNodeID      uint64
	pairedPublicKey   ed25519.PublicKey
	writeToken        string
	readToken         string
	jwt               string
	secureSession     string
}

// relationshipCaller is a caller of the controller that is not a node.
type relationshipCaller struct {
	name, token, secureSession string
}

func (f *relationshipFixture) callers() []relationshipCaller {
	return []relationshipCaller{
		{name: "MFA administrator without step-up", token: f.jwt},
		{name: "MFA administrator stepped up", token: f.jwt, secureSession: f.secureSession},
		{name: "api:write service token", token: f.writeToken},
		{name: "api:read service token", token: f.readToken},
	}
}

// mountRelationshipAPI mounts the given local routers behind AuthRequired and
// router.ProxiedRouters behind AuthRequired and Proxy, as router.InitRouter
// does.
func mountRelationshipAPI(engine *gin.Engine, local []func(*gin.RouterGroup)) {
	api := engine.Group("/api")
	localGroup := api.Group("/", middleware.AuthRequired())
	for _, register := range local {
		register(localGroup)
	}
	proxied := api.Group("/", middleware.AuthRequired(), middleware.Proxy())
	for _, register := range router.ProxiedRouters {
		register(proxied)
	}
}

func setupRelationshipFixture(t *testing.T, local []func(*gin.RouterGroup)) *relationshipFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger.Init("fatal")
	t.Setenv("TMPDIR", t.TempDir())
	cache.InitInMemoryCache()

	oldCrypto := settings.CryptoSettings.Secret
	oldInstance := settings.NodeSettings.InstanceID
	oldNodeSecret := settings.NodeSettings.Secret
	oldJWT := cosysettings.AppSettings.JwtSecret
	settings.CryptoSettings.Secret = "relationship-routes-root"
	settings.NodeSettings.InstanceID = relationshipChildInstanceID
	settings.NodeSettings.Secret = "relationship-routes-child-secret"
	cosysettings.AppSettings.JwtSecret = "relationship-routes-jwt"
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = oldCrypto
		settings.NodeSettings.InstanceID = oldInstance
		settings.NodeSettings.Secret = oldNodeSecret
		cosysettings.AppSettings.JwtSecret = oldJWT
		model.Use(nil)
	})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}, &model.NodeCredential{}, &model.NodeControllerCredential{},
		&model.User{}, &model.Passkey{}, &model.AuthToken{}, &model.MCPServiceToken{}))
	model.Use(db)
	query.SetDefault(db)

	f := &relationshipFixture{db: db}
	admin := &model.User{Name: "relationship-mfa-admin", OTPSecret: []byte("enrolled-mfa"), Status: true}
	require.NoError(t, db.Create(admin).Error)
	t.Cleanup(func() { internaluser.InvalidateUserCache(admin.ID) })
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	f.jwt = payload.Token
	f.secureSession = internaluser.SetSecureSessionID(admin.ID)
	_, f.writeToken, err = internalmcp.CreateServiceToken("relationship-write", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)
	_, f.readToken, err = internalmcp.CreateServiceToken("relationship-read", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)

	child := gin.New()
	child.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/node/") {
			f.childNodeRequests.Add(1)
		}
		c.Next()
	})
	mountRelationshipAPI(child, local)
	childServer := httptest.NewServer(child)
	t.Cleanup(childServer.Close)
	f.childURL = childServer.URL

	legacy := &model.Node{Name: "legacy-child", URL: childServer.URL, Enabled: true,
		AuthMethod: model.NodeAuthMethodLegacy, CredentialStatus: model.NodeCredentialStatusActive}
	require.NoError(t, db.Create(legacy).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(legacy.ID), []byte(settings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, db.Model(legacy).Update("encrypted_legacy_secret", encrypted).Error)
	f.legacyNodeID = legacy.ID

	// A relationship that is already paired: the controller signs with its
	// own key and the child stores the matching public key.
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	f.pairedPublicKey = publicKey
	require.NoError(t, db.Create(&model.NodeControllerCredential{CredentialID: relationshipCredentialID,
		ControllerInstanceID: relationshipControllerInstanceID, PublicKey: publicKey,
		Status: model.NodeCredentialStatusActive}).Error)
	paired := &model.Node{Name: "paired-child", URL: childServer.URL, Enabled: true,
		AuthMethod: model.NodeAuthMethodPaired, CredentialStatus: model.NodeCredentialStatusActive}
	require.NoError(t, db.Create(paired).Error)
	encryptedKey, err := nodeauth.EncryptPrivateCredential(nodeauth.SigningCredentialPurpose(relationshipCredentialID), privateKey)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.NodeCredential{NodeID: paired.ID, CredentialID: relationshipCredentialID,
		TargetInstanceID: relationshipChildInstanceID, PublicKey: publicKey,
		EncryptedPrivateKey: encryptedKey, Status: model.NodeCredentialStatusActive}).Error)
	f.pairedNodeID = paired.ID

	f.controller = gin.New()
	mountRelationshipAPI(f.controller, local)
	return f
}

// viaController sends a request to the controller that selects nodeID.
func (f *relationshipFixture) viaController(t *testing.T, caller relationshipCaller, path string, nodeID uint64, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+caller.token)
	if caller.secureSession != "" {
		req.Header.Set("X-Secure-Session-ID", caller.secureSession)
	}
	req.Header.Set("X-Node-ID", strconv.FormatUint(nodeID, 10))
	res := httptest.NewRecorder()
	f.controller.ServeHTTP(res, req)
	return res
}

func (f *relationshipFixture) controllerCredentials(t *testing.T) []model.NodeControllerCredential {
	t.Helper()
	var credentials []model.NodeControllerCredential
	require.NoError(t, f.db.Unscoped().Order("id").Find(&credentials).Error)
	return credentials
}

func newRelationshipPublicKey(t *testing.T) string {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(publicKey)
}

// assertForwardedRelationshipCallsRefused checks that no caller of the
// controller can reach the relationship endpoints of a child node through the
// node proxy, whatever its session or token scope.
func assertForwardedRelationshipCallsRefused(t *testing.T, f *relationshipFixture, check func(t *testing.T, res *httptest.ResponseRecorder)) {
	t.Helper()
	before := f.controllerCredentials(t)
	for _, caller := range f.callers() {
		t.Run(caller.name, func(t *testing.T) {
			requests := []struct {
				path   string
				nodeID uint64
				body   string
			}{
				{"/api/node/pair/upgrade", f.legacyNodeID,
					`{"controller_instance_id":"dddddddd-dddd-4ddd-8ddd-dddddddddddd","public_key":"` + newRelationshipPublicKey(t) + `"}`},
				{"/api/node/pair/upgrade", f.legacyNodeID,
					`{"controller_instance_id":"` + relationshipControllerInstanceID + `","public_key":"` + newRelationshipPublicKey(t) + `"}`},
				{"/api/node/credentials/" + relationshipCredentialID + "/rotation", f.pairedNodeID,
					`{"public_key":"` + newRelationshipPublicKey(t) + `"}`},
				{"/api/node/credentials/" + relationshipCredentialID + "/rotation/confirm", f.pairedNodeID, ""},
			}
			for index, request := range requests {
				t.Run(strconv.Itoa(index)+" "+request.path, func(t *testing.T) {
					f.childNodeRequests.Store(0)
					res := f.viaController(t, caller, request.path, request.nodeID, request.body)
					require.GreaterOrEqual(t, res.Code, http.StatusBadRequest, res.Body.String())
					require.Zero(t, f.childNodeRequests.Load(), "the request reached the child node")
					if check != nil {
						check(t, res)
					}
				})
			}
		})
	}
	require.Equal(t, before, f.controllerCredentials(t), "the child's controller credentials changed")
}

// GHSA-3m8v-354g-68qf: the node proxy signs a forwarded request as the
// controller, so the relationship endpoints must never be forwarded. This
// mounts only router.ProxiedRouters and checks that none of them serves a
// relationship endpoint for a remote node.
func TestProxiedRoutersDoNotForwardRelationshipEndpoints(t *testing.T) {
	f := setupRelationshipFixture(t, nil)
	assertForwardedRelationshipCallsRefused(t, f, nil)

	var credential model.NodeControllerCredential
	require.NoError(t, f.db.Where("credential_id = ?", relationshipCredentialID).First(&credential).Error)
	require.Equal(t, []byte(f.pairedPublicKey), credential.PublicKey)
	require.Empty(t, credential.PendingPublicKey)
	require.Equal(t, model.NodeCredentialStatusActive, credential.Status)
}
