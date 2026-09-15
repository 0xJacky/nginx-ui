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
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type OIDCLoginUser struct {
	Code  string `form:"code" json:"code" uri:"code" binding:"max=255"`
	State string `form:"state" json:"state" uri:"state" binding:"max=255"`
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

	var username string

	if settings.OIDCSettings.Identifier != "" {
		if v, ok := claims[settings.OIDCSettings.Identifier]; ok {
			username, _ = v.(string)
		}
	}

	if username == "" {
		if v, ok := claims["email"]; ok {
			username, _ = v.(string)
		}
	}

	if username == "" {
		if v, ok := claims["name"]; ok {
			username, _ = v.(string)
		}
	}

	if username == "" {
		if v, ok := claims["sub"]; ok {
			username, _ = v.(string)
		}
	}

	resolvedBy := ""
	if settings.OIDCSettings.Identifier != "" {
		if _, ok := claims[settings.OIDCSettings.Identifier]; ok {
			resolvedBy = settings.OIDCSettings.Identifier
		}
	}
	if resolvedBy == "" {
		for _, fallback := range []string{"email", "name", "sub"} {
			if _, ok := claims[fallback]; ok {
				resolvedBy = fallback
				break
			}
		}
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

func buildOIDCFrontendLoginRedirect(redirectUri string, token string) string {
	parsed, err := url.Parse(redirectUri)
	if err != nil {
		return "/login?oidc_token=" + url.QueryEscape(token)
	}

	parsed.Path = strings.TrimSuffix(parsed.Path, "/api/oidc_callback") + "/login"
	query := parsed.Query()
	query.Set("oidc_token", token)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func buildOIDCFrontendLoginErrorRedirect(redirectUri string, message string) string {
	parsed, err := url.Parse(redirectUri)
	if err != nil {
		return "/login?sso_error=" + url.QueryEscape(message)
	}

	parsed.Path = strings.TrimSuffix(parsed.Path, "/api/oidc_callback") + "/login"
	query := parsed.Query()
	query.Set("sso_error", message)
	parsed.RawQuery = query.Encode()

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
