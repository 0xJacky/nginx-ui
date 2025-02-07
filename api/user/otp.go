package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
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
		api.ErrHandler(c, err)
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

	var json struct {
		Secret   string `json:"secret" binding:"required"`
		Passcode string `json:"passcode" binding:"required"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	if ok := totp.Validate(json.Passcode, json.Secret); !ok {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Invalid passcode",
		})
		return
	}

	ciphertext, err := crypto.AesEncrypt([]byte(json.Secret))
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	u := query.User
	_, err = u.Where(u.ID.Eq(cUser.ID)).Update(u.OTPSecret, ciphertext)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	recoveryCode := sha1.Sum(ciphertext)

	c.JSON(http.StatusOK, gin.H{
		"message":       "ok",
		"recovery_code": hex.EncodeToString(recoveryCode[:]),
	})
}

func ResetOTP(c *gin.Context) {
	var json struct {
		RecoveryCode string `json:"recovery_code"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}
	recoverCode, err := hex.DecodeString(json.RecoveryCode)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	cUser := api.CurrentUser(c)
	k := sha1.Sum(cUser.OTPSecret)
	if !bytes.Equal(k[:], recoverCode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid recovery code",
		})
		return
	}

	u := query.User
	_, err = u.Where(u.ID.Eq(cUser.ID)).UpdateSimple(u.OTPSecret.Null())
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
