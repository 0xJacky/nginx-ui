package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// OIDCLoginUser carries the auth-code callback payload. Neither field is
// length-capped: OAuth 2.0 sets no limit on the authorization code, and real
// IdPs (e.g. Microsoft Entra ID, ~1300 chars) exceed any fixed bound. Capping
// the code here made ShouldBind fail on long codes, which then fell back to the
// (empty) query params and surfaced as a misleading "Missing code or state".
// State is still verified against the oidc_state cookie below, so no length
// limit is needed for safety.
type OIDCLoginUser struct {
	Code  string `form:"code" json:"code" uri:"code"`
	State string `form:"state" json:"state" uri:"state"`
}

func newOIDCRequestContext(ctx context.Context) (context.Context, error) {
	if strings.TrimSpace(settings.HTTPSettings.HTTPProxy) == "" {
		return ctx, nil
	}

	httpTransport, err := transport.NewTransport(transport.WithProxy(settings.HTTPSettings.HTTPProxy))
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: httpTransport}

	return context.WithValue(ctx, oauth2.HTTPClient, httpClient), nil
}

func respondOIDCError(c *gin.Context, redirectUri string, status int, message string, extra gin.H) {
	if c.Request.Method == http.MethodGet {
		c.Redirect(http.StatusFound, buildOIDCFrontendLoginErrorRedirect(redirectUri, message))
		return
	}

	payload := gin.H{"message": message}
	for key, value := range extra {
		payload[key] = value
	}

	c.JSON(status, payload)
}

func respondOIDCCosyError(c *gin.Context, redirectUri string, err error) {
	if c.Request.Method == http.MethodGet {
		c.Redirect(http.StatusFound, buildOIDCFrontendLoginErrorRedirect(redirectUri, "SSO login failed"))
		return
	}

	cosy.ErrHandler(c, err)
}

func OIDCCallback(c *gin.Context) {
	var loginUser OIDCLoginUser
	redirectUri := settings.OIDCSettings.RedirectUri

	if err := c.ShouldBind(&loginUser); err != nil {
		loginUser.Code = c.Query("code")
		loginUser.State = c.Query("state")
	}

	if loginUser.Code == "" || loginUser.State == "" {
		respondOIDCError(c, redirectUri, http.StatusBadRequest, "Missing code or state", nil)
		return
	}

	state, err := c.Cookie("oidc_state")
	if err != nil {
		respondOIDCError(c, redirectUri, http.StatusBadRequest, "State cookie not found", nil)
		return
	}

	if state != loginUser.State {
		respondOIDCError(c, redirectUri, http.StatusForbidden, "State mismatch", nil)
		return
	}

	c.SetCookie("oidc_state", "", -1, "/", "", cSettings.ServerSettings.EnableHTTPS, true)

	endpoint := settings.OIDCSettings.Endpoint
	clientId := settings.OIDCSettings.ClientId
	clientSecret := settings.OIDCSettings.ClientSecret

	if endpoint == "" || clientId == "" || clientSecret == "" || redirectUri == "" {
		respondOIDCError(c, redirectUri, http.StatusInternalServerError, "OIDC is not configured", nil)
		return
	}

	ctx := context.Background()
	ctx, err = newOIDCRequestContext(ctx)
	if err != nil {
		respondOIDCCosyError(c, redirectUri, err)
		return
	}

	provider, err := oidc.NewProvider(ctx, endpoint)
	if err != nil {
		respondOIDCCosyError(c, redirectUri, err)
		return
	}

	scopes := []string{oidc.ScopeOpenID, "profile", "email"}
	if settings.OIDCSettings.Scopes != "" {
		scopes = strings.Split(settings.OIDCSettings.Scopes, " ")
	}

	oauth2Config := oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	oauth2Token, err := oauth2Config.Exchange(ctx, loginUser.Code)
	if err != nil {
		respondOIDCCosyError(c, redirectUri, err)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		respondOIDCError(c, redirectUri, http.StatusInternalServerError, "No id_token field in oauth2 token", nil)
		return
	}

	idTokenVerifier := provider.Verifier(&oidc.Config{ClientID: clientId})
	idToken, err := idTokenVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		respondOIDCCosyError(c, redirectUri, err)
		return
	}

	var claims map[string]interface{}

	if err := idToken.Claims(&claims); err != nil {
		respondOIDCCosyError(c, redirectUri, err)
		return
	}

	username, resolvedBy, rejectReason := resolveOIDCUsername(claims)
	if rejectReason != "" {
		respondOIDCError(c, redirectUri, http.StatusForbidden, rejectReason, gin.H{
			"identifier":        settings.OIDCSettings.Identifier,
			"resolved_by":       resolvedBy,
			"resolved_username": username,
		})
		return
	}

	u, err := user.GetUser(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondOIDCError(c, redirectUri, http.StatusForbidden, "User not exist", gin.H{
				"identifier":        settings.OIDCSettings.Identifier,
				"resolved_by":       resolvedBy,
				"resolved_username": username,
			})
		} else {
			respondOIDCCosyError(c, redirectUri, err)
		}
		return
	}

	if beginRequiredMFA(c, u, user.LoginProofExternal) {
		return
	}
	userToken, err := user.IssueLoginToken(u, user.LoginProofExternal)
	if err != nil {
		respondOIDCCosyError(c, redirectUri, err)
		return
	}

	middleware.EnsureSecureSessionCookie(c)

	if c.Request.Method == http.MethodGet {
		c.Redirect(http.StatusFound, buildOIDCFrontendLoginRedirect(redirectUri, userToken.Token))
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Message:            "ok",
		AccessTokenPayload: userToken,
	})
}

// oidcDefaultIdentifierClaims is the claim order used when no Identifier is
// configured. Only claims the IdP controls are listed: display names and
// preferred_username can be chosen or changed by the end user at many IdPs.
var oidcDefaultIdentifierClaims = []string{"email", "sub"}

// resolveOIDCUsername selects the local username from verified ID token
// claims. A configured Identifier is used exclusively; otherwise the first
// present default claim is used. The email claim is only trusted when the IdP
// asserts email_verified, unless the operator opted out. A non-empty
// rejectReason means the login must be refused.
func resolveOIDCUsername(claims map[string]any) (username, resolvedBy, rejectReason string) {
	if identifier := settings.OIDCSettings.Identifier; identifier != "" {
		username, _ = claims[identifier].(string)
		if username == "" {
			return "", identifier, "OIDC claim \"" + identifier + "\" configured as identifier is missing or not a non-empty string"
		}
		resolvedBy = identifier
	} else {
		for _, claim := range oidcDefaultIdentifierClaims {
			if v, _ := claims[claim].(string); v != "" {
				username, resolvedBy = v, claim
				break
			}
		}
		if username == "" {
			return "", "", "OIDC token has no usable identifier claim"
		}
	}

	if resolvedBy == "email" && !settings.OIDCSettings.TrustUnverifiedEmail && !oidcEmailVerified(claims) {
		return username, resolvedBy, "OIDC email claim is not verified (email_verified is not true)"
	}

	return username, resolvedBy, ""
}

// oidcEmailVerified accepts both the boolean form required by OIDC Core and
// the string form some IdPs send.
func oidcEmailVerified(claims map[string]any) bool {
	switch v := claims["email_verified"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return false
}

func buildOIDCFrontendLoginRedirect(redirectUri string, token string) string {
	return buildOIDCFrontendRedirect(redirectUri, "oidc_token", token)
}

func buildOIDCFrontendLoginRedirectWithQuery(key, value string) string {
	return buildOIDCFrontendRedirect(settings.OIDCSettings.RedirectUri, key, value)
}

func buildOIDCFrontendRedirect(redirectUri, key, value string) string {
	parsed, err := url.Parse(redirectUri)
	if err != nil {
		return "/#/login?" + url.QueryEscape(key) + "=" + url.QueryEscape(value)
	}

	basePath := strings.TrimSuffix(parsed.Path, "/api/oidc_callback")
	if basePath == "" {
		basePath = "/"
	}

	query := url.Values{}
	query.Set(key, value)

	parsed.Path = basePath
	parsed.RawQuery = ""
	parsed.Fragment = "/login?" + query.Encode()

	return parsed.String()
}

func buildOIDCFrontendLoginErrorRedirect(redirectUri string, message string) string {
	parsed, err := url.Parse(redirectUri)
	if err != nil {
		return "/#/login?sso_error=" + url.QueryEscape(message)
	}

	basePath := strings.TrimSuffix(parsed.Path, "/api/oidc_callback")
	if basePath == "" {
		basePath = "/"
	}

	query := url.Values{}
	query.Set("sso_error", message)

	parsed.Path = basePath
	parsed.RawQuery = ""
	parsed.Fragment = "/login?" + query.Encode()

	return parsed.String()
}

func GetOIDCUri(c *gin.Context) {
	endpoint := settings.OIDCSettings.Endpoint
	clientId := settings.OIDCSettings.ClientId
	redirectUri := settings.OIDCSettings.RedirectUri
	scopes := settings.OIDCSettings.Scopes

	if endpoint == "" || clientId == "" || redirectUri == "" {
		c.JSON(http.StatusOK, gin.H{
			"uri": "",
		})
		return
	}

	ctx := context.Background()
	ctx, err := newOIDCRequestContext(ctx)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	provider, err := oidc.NewProvider(ctx, endpoint)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	scopeList := []string{oidc.ScopeOpenID, "profile", "email"}
	if scopes != "" {
		scopeList = strings.Split(scopes, " ")
	}

	oauth2Config := oauth2.Config{
		ClientID:    clientId,
		RedirectURL: redirectUri,
		Endpoint:    provider.Endpoint(),
		Scopes:      scopeList,
	}

	b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	state := "nginx-ui-oidc_" + hex.EncodeToString(b)

	c.SetCookie("oidc_state", state, 300, "/", "", cSettings.ServerSettings.EnableHTTPS, true)

	c.JSON(http.StatusOK, gin.H{
		"uri": oauth2Config.AuthCodeURL(state),
	})
}
