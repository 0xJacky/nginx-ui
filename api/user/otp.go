package user

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

func GenerateTOTP(c *gin.Context) {
	u := api.CurrentUser(c)
	audit.MarkSensitiveResponse(c)
	key, err := generateTOTPKey(u)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"secret": key.Secret(), "url": key.URL()})
}

func generateTOTPKey(u *model.User) (*otp.Key, error) {
	issuer := fmt.Sprintf("Nginx UI %s", settings.NodeSettings.Name)
	issuer = strings.TrimSpace(issuer)

	otpOpts := totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: u.Name,
		Period:      30, // seconds
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	}
	return totp.Generate(otpOpts)
}

func EnrollTOTP(c *gin.Context) {
	cUser := api.CurrentUser(c)
	audit.MarkSensitiveRequest(c)
	audit.MarkSensitiveResponse(c)

	if settings.NodeSettings.Demo {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "This feature is disabled in demo mode",
		})
		return
	}

	var twoFA struct {
		Secret   string `json:"secret" binding:"required"`
		Passcode string `json:"passcode" binding:"required"`
		Password string `json:"password" binding:"required"`
		Replace  bool   `json:"replace"`
	}
	if !cosy.BindAndValid(c, &twoFA) {
		return
	}

	if cUser.EnabledOTP() && !twoFA.Replace {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "User already enrolled"})
		return
	}
	if !verifyCurrentPassword(c, twoFA.Password) {
		return
	}

	version := cUser.MFAVersion
	unlock := user.LockMFA(cUser.ID)
	defer unlock()
	if err := model.UseDB().First(cUser, cUser.ID).Error; err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if cUser.MFAVersion != version {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	if cUser.EnabledOTP() && !twoFA.Replace {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "User already enrolled"})
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

	recoveryCodes, err := newMFARecoveryCodes()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	cUser.OTPSecret, cUser.RecoveryCodes = ciphertext, recoveryCodes
	if err := user.SaveMFACredentials(cUser, func(tx *gorm.DB) error {
		return tx.Model(cUser).Select("otp_secret", "recovery_codes").Updates(cUser).Error
	}); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	user.InvalidateUserCache(cUser.ID)

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: recoveryCodes,
	})
}

func ResetOTP(c *gin.Context) {
	cUser := api.CurrentUser(c)
	err := user.ChangeMFA(cUser.ID, func(tx *gorm.DB, u *model.User) error {
		u.OTPSecret = nil
		return tx.Model(u).Select("otp_secret").Updates(u).Error
	}, cUser.MFAVersion)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	user.InvalidateUserCache(cUser.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
