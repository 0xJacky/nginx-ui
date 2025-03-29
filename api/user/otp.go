package user

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/uozi-tech/cosy"
)

func GenerateTOTP(c *gin.Context) {
	u := api.CurrentUser(c)

	issuer := fmt.Sprintf("Nginx UI %s", settings.NodeSettings.Name)
	issuer = strings.TrimSpace(issuer)

	otpOpts := totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: u.Name,
		Period:      30, // seconds
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	}
	otpKey, err := totp.Generate(otpOpts)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": otpKey.Secret(),
		"url":    otpKey.URL(),
	})
}

func EnrollTOTP(c *gin.Context) {
	cUser := api.CurrentUser(c)
	if cUser.EnabledOTP() {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "User already enrolled",
		})
		return
	}

	if settings.NodeSettings.Demo {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "This feature is disabled in demo mode",
		})
		return
	}

	var twoFA struct {
		Secret   string `json:"secret" binding:"required"`
		Passcode string `json:"passcode" binding:"required"`
	}
	if !cosy.BindAndValid(c, &twoFA) {
		return
	}

	if ok := totp.Validate(twoFA.Passcode, twoFA.Secret); !ok {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Invalid passcode",
		})
		return
	}

	ciphertext, err := crypto.AesEncrypt([]byte(twoFA.Secret))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	u := query.User
	_, err = u.Where(u.ID.Eq(cUser.ID)).Update(u.OTPSecret, ciphertext)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	t := time.Now().Unix()
	recoveryCodes := model.RecoveryCodes{Codes: generateRecoveryCodes(16), LastViewed: &t}
	cUser.RecoveryCodes = recoveryCodes
	_, err = u.Where(u.ID.Eq(cUser.ID)).Updates(cUser)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: recoveryCodes,
	})
}

func ResetOTP(c *gin.Context) {
	cUser := api.CurrentUser(c)
	u := query.User
	_, err := u.Where(u.ID.Eq(cUser.ID)).UpdateSimple(u.OTPSecret.Null(), u.RecoveryCodes.Null())
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
