package user

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"os"
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
		api.ErrHandler(c, err)
		return
	}

	casdoorsdk.InitConfig(endpoint, clientId, clientSecret, string(certBytes), organization, application)

	token, err := casdoorsdk.GetOAuthToken(loginUser.Code, loginUser.State)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	claims, err := casdoorsdk.ParseJwtToken(token.AccessToken)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	u, err := user.GetUser(claims.Name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "User not exist",
			})
		} else {
			api.ErrHandler(c, err)
		}
		return
	}

	userToken, err := user.GenerateJWT(u)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Message: "ok",
		Token:   userToken,
	})
}

func GetCasdoorUri(c *gin.Context) {
	endpoint := settings.CasdoorSettings.Endpoint
	clientId := settings.CasdoorSettings.ClientId
	redirectUri := settings.CasdoorSettings.RedirectUri
	state := settings.CasdoorSettings.Application

	if endpoint == "" || clientId == "" || redirectUri == "" || state == "" {
		c.JSON(http.StatusOK, gin.H{
			"uri": "",
		})
		return
	}

	encodedRedirectUri := url.QueryEscape(redirectUri)

	c.JSON(http.StatusOK, gin.H{
		"uri": fmt.Sprintf(
			"%s/login/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=read",
			endpoint, clientId, encodedRedirectUri, state),
	})
}
