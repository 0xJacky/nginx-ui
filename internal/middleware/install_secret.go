package middleware

import (
	internalSystem "github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func getInstallSecret(c *gin.Context) string {
	if secret := c.GetHeader(internalSystem.InstallSecretHeaderName); secret != "" {
		return secret
	}

	return c.Query(internalSystem.InstallSecretQueryKey)
}

func authorizeWithInstallSecret(c *gin.Context, secret string) error {
	if err := internalSystem.ValidateInstallSecret(secret); err != nil {
		return err
	}

	initUser := &model.User{
		Model: model.Model{
			ID: 1,
		},
		Name:   "admin",
		Status: true,
	}
	if db := model.UseDB(); db != nil {
		loadedUser := &model.User{}
		if err := db.Where("id = ?", initUser.ID).First(loadedUser).Error; err == nil && loadedUser.ID != 0 {
			initUser = loadedUser
		}
	}

	c.Set("InstallSecret", secret)
	c.Set("user", initUser)

	return nil
}

// SetupAuthRequired authorizes first-run setup requests with the one-time install secret.
func SetupAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if internalSystem.InstallLockStatus() {
			cosy.ErrHandler(c, internalSystem.ErrInstalled)
			c.Abort()
			return
		}

		if internalSystem.IsInstallTimeoutExceeded() {
			cosy.ErrHandler(c, internalSystem.ErrInstallTimeout)
			c.Abort()
			return
		}

		secret := getInstallSecret(c)
		if err := authorizeWithInstallSecret(c, secret); err != nil {
			cosy.ErrHandler(c, err)
			c.Abort()
			return
		}

		c.Next()
	}
}
