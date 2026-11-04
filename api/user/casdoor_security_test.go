package user

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
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
	casdoorTestClientID     = "casdoor-test-client"
	casdoorTestClientSecret = "casdoor-test-secret"
	casdoorTestOrganization = "nginx-ui-org"
	casdoorTestApplication  = "nginx-ui-app"
	casdoorTestCode         = "casdoor-test-code"
	casdoorTestState        = "nginx-ui-casdoor_verify"
	casdoorTestUserID       = "9f1c0a52-6a0e-4c43-9c57-6f0d5c8e2b11"
)

// casdoorTestUser is the Casdoor account a test token is minted for.
type casdoorTestUser struct {
	Owner       string
	Name        string
	DisplayName string
}

// The builders below mirror the access token payloads Casdoor signs for each
// application token format (object/token_jwt.go and object/token_standard_jwt.go
// in the Casdoor server).

func casdoorRegisteredClaims(aud any, tokenType string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":       "https://casdoor.test",
		"sub":       casdoorTestUserID,
		"aud":       aud,
		"exp":       now.Add(5 * time.Minute).Unix(),
		"nbf":       now.Unix(),
		"iat":       now.Unix(),
		"jti":       "admin/" + casdoorTestUserID,
		"tokenType": tokenType,
		"azp":       casdoorTestClientID,
		"scope":     "read",
	}
}

// casdoorJWTClaims mirrors the default "JWT" format: the full user object
// with "name" as the username and "displayName" as the display name.
func casdoorJWTClaims(u casdoorTestUser, aud any, tokenType string) jwt.MapClaims {
	claims := casdoorRegisteredClaims(aud, tokenType)
	claims["owner"] = u.Owner
	claims["name"] = u.Name
	claims["id"] = casdoorTestUserID
	claims["displayName"] = u.DisplayName
	claims["email"] = u.Name + "@example.com"
	claims["type"] = "normal-user"
	claims["isAdmin"] = false
	claims["tag"] = ""
	return claims
}

// casdoorJWTEmptyClaims mirrors "JWT-Empty" (UserShort).
func casdoorJWTEmptyClaims(u casdoorTestUser, aud any, tokenType string) jwt.MapClaims {
	claims := casdoorRegisteredClaims(aud, tokenType)
	claims["owner"] = u.Owner
	claims["name"] = u.Name
	claims["id"] = casdoorTestUserID
	claims["displayName"] = u.DisplayName
	claims["avatar"] = ""
	claims["email"] = u.Name + "@example.com"
	claims["phone"] = ""
	return claims
}

// casdoorJWTStandardClaims mirrors "JWT-Standard" (UserStandard): the
// username is "preferred_username" and "name" carries the display name.
func casdoorJWTStandardClaims(u casdoorTestUser, aud any, tokenType string) jwt.MapClaims {
	claims := casdoorRegisteredClaims(aud, tokenType)
	claims["owner"] = u.Owner
	claims["preferred_username"] = u.Name
	claims["id"] = casdoorTestUserID
	claims["name"] = u.DisplayName
	claims["email"] = u.Name + "@example.com"
	return claims
}

// casdoorJWTCustomClaims mirrors "JWT-Custom" with the given token fields
// selected; Casdoor names each claim after the camelCased field.
func casdoorJWTCustomClaims(u casdoorTestUser, aud any, tokenType string, fields ...string) jwt.MapClaims {
	claims := casdoorRegisteredClaims(aud, tokenType)
	delete(claims, "azp")
	claims["nonce"] = ""
	for _, field := range fields {
		switch field {
		case "Owner":
			claims["owner"] = u.Owner
		case "Name":
			claims["name"] = u.Name
		case "DisplayName":
			claims["displayName"] = u.DisplayName
		}
	}
	return claims
}

// casdoorTestKeyPair writes a self-signed certificate to a temp file, the
// way Casdoor exports an application certificate, and returns its signing key.
func casdoorTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "casdoor-test-cert"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "casdoor.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	return key, path
}

// newTestCasdoorServer starts a loopback Casdoor token endpoint that exchanges
// the test code for an access token signed with key and carrying claims.
func newTestCasdoorServer(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("code") != casdoorTestCode ||
			r.PostForm.Get("client_id") != casdoorTestClientID ||
			r.PostForm.Get("client_secret") != casdoorTestClientSecret {
			http.Error(w, "unexpected token request", http.StatusBadRequest)
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "cert-built-in"
		signed, err := token.SignedString(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  signed,
			"id_token":      signed,
			"refresh_token": "test-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    300,
			"scope":         "read",
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// setupCasdoorSecurityTest wires an isolated database, cache and Casdoor
// settings pointing at a loopback token endpoint that returns claims, and
// creates the given local users.
func setupCasdoorSecurityTest(t *testing.T, claims jwt.MapClaims, users ...*model.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	restoreCasdoorTestSettings(t)

	previousAuth := *settings.AuthSettings
	previousSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "casdoor-security-test-secret"
	cSettings.ServerSettings.EnableHTTPS = false
	cache.InitInMemoryCache()
	t.Cleanup(func() {
		cache.Shutdown()
		*settings.AuthSettings = previousAuth
		cSettings.AppSettings.JwtSecret = previousSecret
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

	key, certPath := casdoorTestKeyPair(t)
	srv := newTestCasdoorServer(t, key, claims)
	*settings.CasdoorSettings = settings.Casdoor{
		Endpoint:        srv.URL,
		ClientId:        casdoorTestClientID,
		ClientSecret:    casdoorTestClientSecret,
		CertificatePath: certPath,
		Organization:    casdoorTestOrganization,
		Application:     casdoorTestApplication,
		RedirectUri:     "http://127.0.0.1/",
	}
}

// issueCasdoorCallback posts the callback payload with a matching state cookie.
func issueCasdoorCallback(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.POST("/api/casdoor_callback", CasdoorCallback)

	body := bytes.NewBufferString(fmt.Sprintf(`{"code":%q,"state":%q}`, casdoorTestCode, casdoorTestState))
	req := httptest.NewRequest(http.MethodPost, "/api/casdoor_callback", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: casdoorStateCookie, Value: casdoorTestState})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func casdoorOrgUser(name, displayName string) casdoorTestUser {
	return casdoorTestUser{Owner: casdoorTestOrganization, Name: name, DisplayName: displayName}
}

// A user of another Casdoor organization that shares the application
// certificate must not be mapped to a local account of the same name.
func TestCasdoorCallbackRejectsOtherOrganization(t *testing.T) {
	for _, owner := range []string{"built-in", "other-org"} {
		t.Run(owner, func(t *testing.T) {
			u := casdoorTestUser{Owner: owner, Name: "admin", DisplayName: "admin"}
			setupCasdoorSecurityTest(t, casdoorJWTClaims(u, casdoorTestClientID, "access-token"), adminUser())

			body := requireOIDCRejected(t, issueCasdoorCallback(t))
			assert.Contains(t, body["message"], "organization")
			assert.Equal(t, owner, body["organization"])
		})
	}
}

// A token minted for another Casdoor application signed with the same
// certificate must not be accepted.
func TestCasdoorCallbackRejectsWrongAudience(t *testing.T) {
	for name, aud := range map[string]any{
		"other client":           "other-app-client",
		"other client list":      []string{"other-app-client"},
		"shared app other org":   casdoorTestClientID + "-org-other-org",
		"client id prefix match": casdoorTestClientID + "-extra",
	} {
		t.Run(name, func(t *testing.T) {
			setupCasdoorSecurityTest(t, casdoorJWTClaims(casdoorOrgUser("admin", "admin"), aud, "access-token"), adminUser())

			body := requireOIDCRejected(t, issueCasdoorCallback(t))
			assert.Contains(t, body["message"], "audience")
		})
	}
}

func TestCasdoorCallbackRejectsMissingAudience(t *testing.T) {
	claims := casdoorJWTClaims(casdoorOrgUser("admin", "admin"), casdoorTestClientID, "access-token")
	delete(claims, "aud")
	setupCasdoorSecurityTest(t, claims, adminUser())

	body := requireOIDCRejected(t, issueCasdoorCallback(t))
	assert.Contains(t, body["message"], "audience")
}

func TestCasdoorCallbackRejectsRefreshToken(t *testing.T) {
	for _, key := range []string{"tokenType", "TokenType"} {
		t.Run(key, func(t *testing.T) {
			claims := casdoorJWTClaims(casdoorOrgUser("admin", "admin"), casdoorTestClientID, "")
			delete(claims, "tokenType")
			claims[key] = "refresh-token"
			setupCasdoorSecurityTest(t, claims, adminUser())

			body := requireOIDCRejected(t, issueCasdoorCallback(t))
			assert.Contains(t, body["message"], "refresh token")
		})
	}
}

// In the JWT-Standard format "name" is the display name, which Casdoor lets
// users edit themselves by default; only preferred_username may select the
// local account.
func TestCasdoorCallbackJWTStandardDoesNotResolveByDisplayName(t *testing.T) {
	setupCasdoorSecurityTest(t,
		casdoorJWTStandardClaims(casdoorOrgUser("mallory", "admin"), casdoorTestClientID, "access-token"),
		adminUser())

	body := requireOIDCRejected(t, issueCasdoorCallback(t))
	assert.Equal(t, "User not exist", body["message"])
	assert.Equal(t, "preferred_username", body["resolved_by"])
	assert.Equal(t, "mallory", body["resolved_username"])
}

// In formats that carry "displayName", "name" is the username; a
// preferred_username claim there can only come from a custom token attribute
// and must not override it.
func TestCasdoorCallbackIgnoresPreferredUsernameAttributeWithDisplayName(t *testing.T) {
	for format, build := range map[string]func() jwt.MapClaims{
		"JWT": func() jwt.MapClaims {
			return casdoorJWTClaims(casdoorOrgUser("mallory", "Mallory"), casdoorTestClientID, "access-token")
		},
		"JWT-Custom": func() jwt.MapClaims {
			return casdoorJWTCustomClaims(casdoorOrgUser("mallory", "Mallory"), casdoorTestClientID, "access-token",
				"Owner", "Name", "DisplayName")
		},
	} {
		t.Run(format, func(t *testing.T) {
			claims := build()
			claims["preferred_username"] = "admin"
			setupCasdoorSecurityTest(t, claims, adminUser())

			body := requireOIDCRejected(t, issueCasdoorCallback(t))
			assert.Equal(t, "User not exist", body["message"])
			assert.Equal(t, "name", body["resolved_by"])
			assert.Equal(t, "mallory", body["resolved_username"])
		})
	}
}

func TestCasdoorCallbackJWTStandardRejectsInvalidPreferredUsername(t *testing.T) {
	for name, value := range map[string]any{"empty": "", "not a string": 42} {
		t.Run(name, func(t *testing.T) {
			claims := casdoorJWTStandardClaims(casdoorOrgUser("mallory", "admin"), casdoorTestClientID, "access-token")
			claims["preferred_username"] = value
			setupCasdoorSecurityTest(t, claims, adminUser())

			body := requireOIDCRejected(t, issueCasdoorCallback(t))
			assert.Contains(t, body["message"], "preferred_username")
		})
	}
}

// Without the owner claim the organization cannot be verified, so the login
// is refused instead of trusting the username alone.
func TestCasdoorCallbackRejectsTokenWithoutOrganization(t *testing.T) {
	setupCasdoorSecurityTest(t,
		casdoorJWTCustomClaims(casdoorOrgUser("admin", "admin"), casdoorTestClientID, "access-token", "Name"),
		adminUser())

	body := requireOIDCRejected(t, issueCasdoorCallback(t))
	assert.Contains(t, body["message"], "owner")
}

func TestCasdoorCallbackRejectsTokenWithoutUsername(t *testing.T) {
	setupCasdoorSecurityTest(t,
		casdoorJWTCustomClaims(casdoorOrgUser("admin", "admin"), casdoorTestClientID, "access-token", "Owner", "DisplayName"),
		adminUser())

	body := requireOIDCRejected(t, issueCasdoorCallback(t))
	assert.Contains(t, body["message"], "username")
}

func TestCasdoorCallbackLogsInForEachTokenFormat(t *testing.T) {
	alice := casdoorOrgUser("alice", "Alice Example")
	for name, claims := range map[string]jwt.MapClaims{
		"JWT":          casdoorJWTClaims(alice, casdoorTestClientID, "access-token"),
		"JWT-Empty":    casdoorJWTEmptyClaims(alice, casdoorTestClientID, "access-token"),
		"JWT-Standard": casdoorJWTStandardClaims(alice, casdoorTestClientID, "access-token"),
		"JWT-Custom":   casdoorJWTCustomClaims(alice, casdoorTestClientID, "access-token", "Owner", "Name"),
		"JWT-Custom with display name": casdoorJWTCustomClaims(alice, casdoorTestClientID, "access-token",
			"Owner", "Name", "DisplayName"),
	} {
		t.Run(name, func(t *testing.T) {
			setupCasdoorSecurityTest(t, claims, adminUser(), &model.User{Name: "alice", Status: true})

			requireOIDCLoginAs(t, issueCasdoorCallback(t), "alice")
		})
	}
}

// Casdoor versions that predate the tokenType claim still log in.
func TestCasdoorCallbackLogsInWithoutTokenType(t *testing.T) {
	claims := casdoorJWTClaims(casdoorOrgUser("alice", "Alice"), casdoorTestClientID, "")
	delete(claims, "tokenType")
	setupCasdoorSecurityTest(t, claims, &model.User{Name: "alice", Status: true})

	requireOIDCLoginAs(t, issueCasdoorCallback(t), "alice")
}

// Shared Casdoor applications address the token to "<clientId>-org-<owner>".
func TestCasdoorCallbackAcceptsAudienceForms(t *testing.T) {
	for name, aud := range map[string]any{
		"list":       []string{"other-app-client", casdoorTestClientID},
		"shared app": casdoorTestClientID + "-org-" + casdoorTestOrganization,
	} {
		t.Run(name, func(t *testing.T) {
			setupCasdoorSecurityTest(t,
				casdoorJWTClaims(casdoorOrgUser("alice", "Alice"), aud, "access-token"),
				&model.User{Name: "alice", Status: true})

			requireOIDCLoginAs(t, issueCasdoorCallback(t), "alice")
		})
	}
}

func TestCasdoorCallbackRejectsUnknownUser(t *testing.T) {
	setupCasdoorSecurityTest(t,
		casdoorJWTClaims(casdoorOrgUser("bob", "Bob"), casdoorTestClientID, "access-token"),
		adminUser())

	body := requireOIDCRejected(t, issueCasdoorCallback(t))
	assert.Equal(t, "User not exist", body["message"])
}

func TestCasdoorCallbackStillRequiresMFAForEnrolledUser(t *testing.T) {
	setupCasdoorSecurityTest(t,
		casdoorJWTClaims(casdoorOrgUser("alice", "Alice"), casdoorTestClientID, "access-token"),
		&model.User{Name: "alice", Status: true, OTPSecret: []byte("otp-enabled")})
	settings.AuthSettings.MFARequiredForSSO = true

	recorder := issueCasdoorCallback(t)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := decodeOIDCResponse(t, recorder)
	assert.Equal(t, "verify", body["mfa_stage"])
	assert.NotEmpty(t, body["pre_auth_id"])
	assert.Empty(t, body["token"])
}
