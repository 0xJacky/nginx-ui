package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"image/jpeg"
	"net/http"
	"strings"
)

func GenerateTOTP(c *gin.Context) {
	user := api.CurrentUser(c)

	issuer := fmt.Sprintf("Nginx UI %s", settings.ServerSettings.Name)
	issuer = strings.TrimSpace(issuer)

	otpOpts := totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: user.Name,
		Period:      30, // seconds
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	}
	otpKey, err := totp.Generate(otpOpts)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	ciphertext, err := crypto.AesEncrypt([]byte(otpKey.Secret()))
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	qrCode, err := otpKey.Image(512, 512)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	// Encode the image to a buffer
	var buf []byte
	buffer := bytes.NewBuffer(buf)
	err = jpeg.Encode(buffer, qrCode, nil)
	if err != nil {
		fmt.Println("Error encoding image:", err)
		return
	}

	// Convert the buffer to a base64 string
	base64Str := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())

	c.JSON(http.StatusOK, gin.H{
		"secret":  base64.StdEncoding.EncodeToString(ciphertext),
		"qr_code": base64Str,
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

	var json struct {
		Secret   string `json:"secret" binding:"required"`
		Passcode string `json:"passcode" binding:"required"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}

	secret, err := base64.StdEncoding.DecodeString(json.Secret)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	decrypted, err := crypto.AesDecrypt(secret)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if ok := totp.Validate(json.Passcode, string(decrypted)); !ok {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Invalid passcode",
		})
		return
	}

	ciphertext, err := crypto.AesEncrypt(decrypted)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	u := query.Auth
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
	if !api.BindAndValid(c, &json) {
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

	u := query.Auth
	_, err = u.Where(u.ID.Eq(cUser.ID)).UpdateSimple(u.OTPSecret.Null())
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func OTPStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": len(api.CurrentUser(c).OTPSecret) > 0,
	})
}

func StartSecure2FASession(c *gin.Context) {
	var json struct {
		OTP          string `json:"otp"`
		RecoveryCode string `json:"recovery_code"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}
	u := api.CurrentUser(c)
	if !u.EnabledOTP() {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "User not configured with 2FA",
		})
		return
	}

	if json.OTP == "" && json.RecoveryCode == "" {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Message: "The user has enabled 2FA",
		})
		return
	}

	if err := user.VerifyOTP(u, json.OTP, json.RecoveryCode); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Message: "Invalid 2FA or recovery code",
		})
		return
	}

	sessionId := user.SetSecureSessionID(u.ID)

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionId,
	})
}
