package user

import (
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"golang.org/x/crypto/bcrypt"
)

func encryptPassword(ctx *cosy.Ctx[model.User]) {
	if ctx.Payload["password"] == nil {
		return
	}
	pwd := ctx.Payload["password"].(string)
	if pwd != "" {
		pwdBytes, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		if err != nil {
			ctx.AbortWithError(err)
			return
		}
		ctx.Model.Password = string(pwdBytes)
	} else {
		delete(ctx.Payload, "password")
	}
}

func InitManageUserRouter(g *gin.RouterGroup) {
	c := cosy.Api[model.User]("users")

	c.CreateHook(func(c *cosy.Ctx[model.User]) {
		c.BeforeDecodeHook(encryptPassword)
	})

	c.ModifyHook(func(c *cosy.Ctx[model.User]) {
		c.BeforeDecodeHook(func(ctx *cosy.Ctx[model.User]) {
			if settings.NodeSettings.Demo && ctx.ID == 1 {
				ctx.AbortWithError(user.ErrChangeInitUserPwdInDemo)
			}
		})
		c.BeforeDecodeHook(encryptPassword)
	})

	c.DestroyHook(func(c *cosy.Ctx[model.User]) {
		c.BeforeExecuteHook(func(ctx *cosy.Ctx[model.User]) {
			if ctx.ID == 1 {
				ctx.AbortWithError(user.ErrCannotRemoveInitUser)
			}
		})
	})

	c.InitRouter(g)
}
