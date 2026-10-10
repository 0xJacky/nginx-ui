package user

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/passkey"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

func BeginMFAPasskey(c *gin.Context) {
	withMFAPreAuth(c, func(s *mfaPreAuth, u *model.User, _ string, stage string) {
		if !passkey.Enabled() {
			cosy.ErrHandler(c, user.ErrWebAuthnNotConfigured)
			return
		}
		s.Registering = stage == "setup"
		if s.Registering && !allowMFAEnrollment(c) {
			return
		}
		if s.Registering {
			options, data, err := passkey.GetInstance().BeginRegistration(u)
			if err != nil {
				cosy.ErrHandler(c, err)
				return
			}
			s.WebAuthn = data
			c.JSON(http.StatusOK, options.Response)
			return
		}
		options, data, err := passkey.GetInstance().BeginLogin(u)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		s.WebAuthn = data
		c.JSON(http.StatusOK, options.Response)
	})
}

func FinishMFAPasskey(c *gin.Context) {
	withMFAPreAuth(c, func(s *mfaPreAuth, u *model.User, id, stage string) {
		if !passkey.Enabled() {
			cosy.ErrHandler(c, user.ErrWebAuthnNotConfigured)
			return
		}
		if s.WebAuthn == nil || s.Registering != (stage == "setup") {
			cosy.ErrHandler(c, user.ErrSessionNotFound)
			return
		}
		data := s.WebAuthn
		s.WebAuthn = nil
		s.Attempts++
		if s.Registering && !allowMFAEnrollment(c) {
			return
		}
		if s.Registering {
			credential, err := passkey.GetInstance().FinishRegistration(u, *data, c.Request)
			if err != nil {
				rejectMFAVerification(c, err)
				return
			}
			codes, err := newMFARecoveryCodes()
			if err != nil {
				cosy.ErrHandler(c, err)
				return
			}
			name := strings.TrimSpace(c.Query("name"))
			if name == "" {
				name = "Passkey"
			}
			if len(name) > 255 {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Passkey name is too long"})
				return
			}
			err = user.SaveMFACredentials(u, func(tx *gorm.DB) error {
				if err := tx.Create(&model.Passkey{UserID: u.ID, Name: name,
					RawID:      strings.TrimRight(base64.StdEncoding.EncodeToString(credential.ID), "="),
					Credential: credential, LastUsedAt: time.Now().Unix(),
				}).Error; err != nil {
					return err
				}
				u.RecoveryCodes = codes
				return tx.Model(u).Select("recovery_codes").Updates(u).Error
			})
			if err != nil {
				cosy.ErrHandler(c, err)
				return
			}
			user.InvalidateUserCache(u.ID)
			completeMFAPreAuth(c, s, u, id, user.LoginProofPasskey, &codes)
			return
		}
		credential, err := passkey.GetInstance().FinishLogin(u, *data, c.Request)
		if err != nil {
			rejectMFAVerification(c, err)
			return
		}
		rawID := strings.TrimRight(base64.StdEncoding.EncodeToString(credential.ID), "=")
		if err := model.UseDB().Model(&model.Passkey{}).Where("user_id = ? AND raw_id = ?", u.ID, rawID).Update("last_used_at", time.Now().Unix()).Error; err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		completeMFAPreAuth(c, s, u, id, user.LoginProofPasskey, nil)
	})
}
