package user

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/gorm"
)

const (
	casdoorStateCookie = "casdoor_state"
	casdoorStateMaxAge = 300
)

type CasdoorLoginUser struct {
	Code  string `json:"code" binding:"required,max=255"`
	State string `json:"state" binding:"required,max=255"`
}

func CasdoorCallback(c *gin.Context) {
	var loginUser CasdoorLoginUser

	ok := cosy.BindAndValid(c, &loginUser)
	if !ok {
		return
	}

	if !validateCasdoorState(c, loginUser.State) {
		return
	}

	endpoint := settings.CasdoorSettings.Endpoint
	clientId := settings.CasdoorSettings.ClientId
	clientSecret := settings.CasdoorSettings.ClientSecret
	certificatePath := settings.CasdoorSettings.CertificatePath
	organization := settings.CasdoorSettings.Organization
	application := settings.CasdoorSettings.Application
	if endpoint == "" || clientId == "" || clientSecret == "" || certificatePath == "" ||
		organization == "" || application == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Casdoor is not configured",
		})
		return
	}

	certBytes, err := os.ReadFile(certificatePath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	casdoorsdk.InitConfig(endpoint, clientId, clientSecret, string(certBytes), organization, application)

	token, err := casdoorsdk.GetOAuthToken(loginUser.Code, loginUser.State)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	claims, err := casdoorsdk.ParseJwtToken(token.AccessToken)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	username, resolvedBy, rejectReason := resolveCasdoorUsername(token.AccessToken, claims, clientId, organization)
	if rejectReason != "" {
		c.JSON(http.StatusForbidden, gin.H{
			"message":           rejectReason,
			"organization":      claims.Owner,
			"resolved_by":       resolvedBy,
			"resolved_username": username,
		})
		return
	}

	u, err := user.GetUser(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{
				"message":           "User not exist",
				"organization":      claims.Owner,
				"resolved_by":       resolvedBy,
				"resolved_username": username,
			})
		} else {
			cosy.ErrHandler(c, err)
		}
		return
	}

	if beginRequiredMFA(c, u, user.LoginProofExternal) {
		return
	}
	userToken, err := user.IssueLoginToken(u, user.LoginProofExternal)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	middleware.EnsureSecureSessionCookie(c)

	c.JSON(http.StatusOK, LoginResponse{
		Message:            "ok",
		AccessTokenPayload: userToken,
	})
}

// resolveCasdoorUsername checks that a signature-verified Casdoor access token
// was issued to the configured application for a user of the configured
// organization, and returns the Casdoor username that selects the local
// account. casdoorsdk.ParseJwtToken only verifies the signature and the time
// claims, and Casdoor applications commonly share one certificate, so the
// audience and owner claims have to be checked here. A non-empty rejectReason
// means the login must be refused.
func resolveCasdoorUsername(accessToken string, claims *casdoorsdk.Claims, clientId, organization string) (username, resolvedBy, rejectReason string) {
	if claims.IsRefreshToken() {
		return "", "", "Casdoor returned a refresh token instead of an access token"
	}

	if !casdoorAudienceAllowed(claims.Audience, clientId, organization) {
		return "", "", "Casdoor token audience does not match the configured client ID"
	}

	if claims.Owner == "" {
		return "", "", "Casdoor token has no organization (owner) claim; use the JWT, JWT-Empty or JWT-Standard " +
			"token format, or select Owner in the JWT-Custom token fields"
	}
	if claims.Owner != organization {
		return "", "", "Casdoor user does not belong to the configured organization"
	}

	return casdoorTokenUsername(accessToken, claims)
}

// casdoorAudienceAllowed accepts the client ID itself and, for a shared
// Casdoor application, "<clientId>-org-<organization>", which is the audience
// Casdoor issues to users of that organization.
func casdoorAudienceAllowed(audience []string, clientId, organization string) bool {
	sharedAudience := clientId + "-org-" + organization
	for _, aud := range audience {
		if aud == clientId || aud == sharedAudience {
			return true
		}
	}
	return false
}

// casdoorTokenUsername returns the unique Casdoor username. The JWT, JWT-Empty
// and JWT-Custom formats carry it in "name", but JWT-Standard follows OIDC and
// carries it in "preferred_username" while "name" holds the display name,
// which users may edit themselves. preferred_username therefore wins when it
// is present and never falls back to "name", except in tokens that carry the
// display name as "displayName": there "name" is the username, and a
// preferred_username claim can only come from a custom token attribute.
func casdoorTokenUsername(accessToken string, claims *casdoorsdk.Claims) (username, resolvedBy, rejectReason string) {
	// The token was verified by casdoorsdk.ParseJwtToken; this only reads the
	// claim the SDK's Claims type does not expose.
	rawClaims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(accessToken, rawClaims); err != nil {
		return "", "", "Casdoor token claims could not be decoded"
	}

	_, hasDisplayName := rawClaims["displayName"]
	if value, ok := rawClaims["preferred_username"]; ok && !hasDisplayName {
		username, _ = value.(string)
		if username == "" {
			return "", "preferred_username", "Casdoor claim \"preferred_username\" is not a non-empty string"
		}
		return username, "preferred_username", ""
	}

	if claims.Name == "" {
		return "", "name", "Casdoor token has no username claim; select Name in the JWT-Custom token fields"
	}
	return claims.Name, "name", ""
}

func validateCasdoorState(c *gin.Context, loginState string) bool {
	state, err := c.Cookie(casdoorStateCookie)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "State cookie not found",
		})
		return false
	}

	if !constantTimeStateEqual(state, loginState) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "State mismatch",
		})
		return false
	}

	setCasdoorStateCookie(c, "", -1)
	return true
}

func constantTimeStateEqual(expected, actual string) bool {
	expectedHash := sha256.Sum256([]byte(expected))
	actualHash := sha256.Sum256([]byte(actual))
	return subtle.ConstantTimeCompare(expectedHash[:], actualHash[:]) == 1
}

func setCasdoorStateCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(casdoorStateCookie, value, maxAge, "/", "", cSettings.ServerSettings.EnableHTTPS, true)
}

func GetCasdoorUri(c *gin.Context) {
	clientId := settings.CasdoorSettings.ClientId
	redirectUri := settings.CasdoorSettings.RedirectUri
	application := settings.CasdoorSettings.Application

	endpoint := settings.CasdoorSettings.Endpoint
	// feature request #603
	if settings.CasdoorSettings.ExternalUrl != "" {
		endpoint = settings.CasdoorSettings.ExternalUrl
	}

	if endpoint == "" || clientId == "" || redirectUri == "" || application == "" {
		c.JSON(http.StatusOK, gin.H{
			"uri": "",
		})
		return
	}

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	state := "nginx-ui-casdoor_" + hex.EncodeToString(b)
	setCasdoorStateCookie(c, state, casdoorStateMaxAge)

	encodedRedirectUri := url.QueryEscape(redirectUri)

	c.JSON(http.StatusOK, gin.H{
		"uri": fmt.Sprintf(
			"%s/login/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=read",
			endpoint, clientId, encodedRedirectUri, state),
	})
}
