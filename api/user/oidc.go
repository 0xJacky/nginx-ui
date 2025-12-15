package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

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
	Code  string `json:"code" binding:"required,max=255"`
	State string `json:"state" binding:"required,max=255"`
}

func OIDCCallback(c *gin.Context) {
	var loginUser OIDCLoginUser

	ok := cosy.BindAndValid(c, &loginUser)
	if !ok {
		return
	}

	state, err := c.Cookie("oidc_state")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "State cookie not found",
		})
		return
	}

	if state != loginUser.State {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "State mismatch",
		})
		return
	}

	c.SetCookie("oidc_state", "", -1, "/", "", cSettings.ServerSettings.EnableHTTPS, true)

	endpoint := settings.OIDCSettings.Endpoint
	clientId := settings.OIDCSettings.ClientId
	clientSecret := settings.OIDCSettings.ClientSecret
	redirectUri := settings.OIDCSettings.RedirectUri

	if endpoint == "" || clientId == "" || clientSecret == "" || redirectUri == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "OIDC is not configured",
		})
		return
	}

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, endpoint)
	if err != nil {
		cosy.ErrHandler(c, err)
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
		cosy.ErrHandler(c, err)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "No id_token field in oauth2 token",
		})
		return
	}

	idTokenVerifier := provider.Verifier(&oidc.Config{ClientID: clientId})
	idToken, err := idTokenVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	var claims map[string]interface{}

	if err := idToken.Claims(&claims); err != nil {
		cosy.ErrHandler(c, err)
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

	u, err := user.GetUser(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "User not exist",
			})
		} else {
			cosy.ErrHandler(c, err)
		}
		return
	}

	userToken, err := user.GenerateJWT(u)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Message:            "ok",
		AccessTokenPayload: userToken,
	})
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
