package user

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	oidcTestClientID = "nginx-ui-test-client"
	oidcTestSubject  = "idp-subject-0001"
	oidcTestKeyID    = "test-key"
	oidcTestState    = "nginx-ui-oidc_verify"
)

// newTestOIDCProvider starts a loopback OIDC provider that signs an RS256
// id_token carrying iss, aud, sub, exp, iat plus the given profile claims.
func newTestOIDCProvider(t *testing.T, profile map[string]any) *httptest.Server {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/authorize",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"keys": []map[string]any{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": oidcTestKeyID,
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		claims := jwt.MapClaims{
			"iss": srv.URL,
			"aud": oidcTestClientID,
			"sub": oidcTestSubject,
			"exp": now.Add(5 * time.Minute).Unix(),
			"iat": now.Unix(),
		}
		for k, v := range profile {
			claims[k] = v
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = oidcTestKeyID
		signed, err := token.SignedString(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   300,
			"id_token":     signed,
		})
	})

	return srv
}

// setupOIDCSecurityTest wires an isolated database, cache and OIDC settings
// pointing at a fresh loopback provider, and creates the given local users.
func setupOIDCSecurityTest(t *testing.T, identifier string, profile map[string]any, users ...*model.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	restoreOIDCSettings(t)

	previousAuth := *settings.AuthSettings
	previousSecret := cSettings.AppSettings.JwtSecret
	previousHTTPS := cSettings.ServerSettings.EnableHTTPS
	cSettings.AppSettings.JwtSecret = "oidc-security-test-secret"
	cSettings.ServerSettings.EnableHTTPS = false
	cache.InitInMemoryCache()
	t.Cleanup(func() {
		cache.Shutdown()
		*settings.AuthSettings = previousAuth
		cSettings.AppSettings.JwtSecret = previousSecret
		cSettings.ServerSettings.EnableHTTPS = previousHTTPS
	})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	for _, u := range users {
		require.NoError(t, db.Create(u).Error)
	}

	srv := newTestOIDCProvider(t, profile)
	*settings.OIDCSettings = settings.OIDC{
		Endpoint:     srv.URL,
		ClientId:     oidcTestClientID,
		ClientSecret: "test-client-secret",
		RedirectUri:  "http://127.0.0.1/",
		Identifier:   identifier,
	}
}

func decodeOIDCResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	return body
}

// requireOIDCLoginAs asserts that the callback issued a session for the named user.
func requireOIDCLoginAs(t *testing.T, recorder *httptest.ResponseRecorder, name string) {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := decodeOIDCResponse(t, recorder)
	token, _ := body["token"].(string)
	require.NotEmpty(t, token, recorder.Body.String())
	u, ok := internaluser.GetTokenUser(token)
	require.True(t, ok)
	assert.Equal(t, name, u.Name)
}

// requireOIDCRejected asserts that no session was issued.
func requireOIDCRejected(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	body := decodeOIDCResponse(t, recorder)
	assert.Empty(t, body["token"])
	return body
}

func adminUser() *model.User {
	return &model.User{Name: "admin", Status: true}
}

// The display name is editable by the end user at most IdPs, so it must never
// select the local account.
func TestOIDCCallbackDoesNotResolveByDisplayName(t *testing.T) {
	setupOIDCSecurityTest(t, "", map[string]any{"name": "admin"}, adminUser())

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Equal(t, "sub", body["resolved_by"])
	assert.Equal(t, oidcTestSubject, body["resolved_username"])
}

// A configured Identifier must not fall back to other claims when its own
// claim is absent.
func TestOIDCCallbackConfiguredIdentifierMissingDoesNotFallBack(t *testing.T) {
	setupOIDCSecurityTest(t, "email", map[string]any{"name": "admin"}, adminUser())

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Contains(t, body["message"], "email")
	assert.Equal(t, "email", body["identifier"])
	assert.Equal(t, "", body["resolved_username"])
}

func TestOIDCCallbackConfiguredIdentifierNotStringDoesNotFallBack(t *testing.T) {
	setupOIDCSecurityTest(t, "upn", map[string]any{"upn": 42, "email": "admin", "email_verified": true}, adminUser())

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Contains(t, body["message"], "upn")
}

func TestOIDCCallbackConfiguredIdentifierEmptyDoesNotFallBack(t *testing.T) {
	setupOIDCSecurityTest(t, "upn", map[string]any{"upn": "", "name": "admin"}, adminUser())

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Contains(t, body["message"], "upn")
}

func TestOIDCCallbackRejectsUnverifiedEmailIdentifier(t *testing.T) {
	for name, verified := range map[string]any{
		"false":        false,
		"string false": "false",
		"absent":       nil,
	} {
		t.Run(name, func(t *testing.T) {
			profile := map[string]any{"email": "admin"}
			if verified != nil {
				profile["email_verified"] = verified
			}
			setupOIDCSecurityTest(t, "email", profile, adminUser())

			body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
			assert.Contains(t, body["message"], "email_verified")
		})
	}
}

func TestOIDCCallbackRejectsUnverifiedEmailFallback(t *testing.T) {
	// A user named after sub exists, so a silent fall-through to sub would log in.
	setupOIDCSecurityTest(t, "", map[string]any{"email": "admin", "email_verified": false},
		adminUser(), &model.User{Name: oidcTestSubject, Status: true})

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Contains(t, body["message"], "email_verified")
}

// preferred_username is not guaranteed unique or immutable (OIDC Core 5.1),
// so it only selects an account when the operator configures it explicitly.
func TestOIDCCallbackDefaultChainIgnoresPreferredUsername(t *testing.T) {
	setupOIDCSecurityTest(t, "", map[string]any{"preferred_username": "admin"}, adminUser())

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Equal(t, "sub", body["resolved_by"])
	assert.Equal(t, oidcTestSubject, body["resolved_username"])
}

func TestOIDCCallbackRejectsWhenNoClaimMatches(t *testing.T) {
	setupOIDCSecurityTest(t, "", map[string]any{"email": "someone@example.com", "email_verified": true}, adminUser())

	body := requireOIDCRejected(t, issueOIDCCallbackJSON(t, "code", oidcTestState))
	assert.Equal(t, "User not exist", body["message"])
	assert.Equal(t, "email", body["resolved_by"])
	assert.Equal(t, "someone@example.com", body["resolved_username"])
}

func TestOIDCCallbackIdentifierSubLogsIn(t *testing.T) {
	setupOIDCSecurityTest(t, "sub", map[string]any{"name": "admin"},
		adminUser(), &model.User{Name: oidcTestSubject, Status: true})

	requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), oidcTestSubject)
}

func TestOIDCCallbackIdentifierEmailVerifiedLogsIn(t *testing.T) {
	for name, verified := range map[string]any{"bool": true, "string": "true"} {
		t.Run(name, func(t *testing.T) {
			setupOIDCSecurityTest(t, "email", map[string]any{"email": "alice@example.com", "email_verified": verified},
				&model.User{Name: "alice@example.com", Status: true})

			requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), "alice@example.com")
		})
	}
}

func TestOIDCCallbackTrustUnverifiedEmailOptOut(t *testing.T) {
	setupOIDCSecurityTest(t, "email", map[string]any{"email": "alice@example.com"},
		&model.User{Name: "alice@example.com", Status: true})
	settings.OIDCSettings.TrustUnverifiedEmail = true

	requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), "alice@example.com")
}

func TestOIDCCallbackIdentifierPreferredUsernameLogsIn(t *testing.T) {
	setupOIDCSecurityTest(t, "preferred_username", map[string]any{"preferred_username": "alice"},
		&model.User{Name: "alice", Status: true})

	requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), "alice")
}

func TestOIDCCallbackDefaultChainVerifiedEmailLogsIn(t *testing.T) {
	setupOIDCSecurityTest(t, "", map[string]any{"email": "alice@example.com", "email_verified": true, "name": "admin"},
		adminUser(), &model.User{Name: "alice@example.com", Status: true})

	requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), "alice@example.com")
}

func TestOIDCCallbackDefaultChainFallsBackToSub(t *testing.T) {
	setupOIDCSecurityTest(t, "", map[string]any{"name": "admin"},
		adminUser(), &model.User{Name: oidcTestSubject, Status: true})

	requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), oidcTestSubject)
}

func TestOIDCCallbackCustomIdentifierLogsIn(t *testing.T) {
	setupOIDCSecurityTest(t, "upn", map[string]any{"upn": "alice@corp.example", "name": "admin"},
		adminUser(), &model.User{Name: "alice@corp.example", Status: true})

	requireOIDCLoginAs(t, issueOIDCCallbackJSON(t, "code", oidcTestState), "alice@corp.example")
}

func TestOIDCCallbackStillRequiresMFAForEnrolledUser(t *testing.T) {
	setupOIDCSecurityTest(t, "sub", nil,
		&model.User{Name: oidcTestSubject, Status: true, OTPSecret: []byte("otp-enabled")})
	settings.AuthSettings.MFARequiredForSSO = true

	recorder := issueOIDCCallbackJSON(t, "code", oidcTestState)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := decodeOIDCResponse(t, recorder)
	assert.Equal(t, "verify", body["mfa_stage"])
	assert.NotEmpty(t, body["pre_auth_id"])
	assert.Empty(t, body["token"])
}
