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

	u, err := user.GetUser(claims.Name)
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

	middleware.EnsureSecureSessionCookie(c)

	c.JSON(http.StatusOK, LoginResponse{
		Message:            "ok",
		AccessTokenPayload: userToken,
	})
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
