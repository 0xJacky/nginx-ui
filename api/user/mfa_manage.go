package user

import (
	"net/http"
	"strconv"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func checkUserMFAPolicy(ctx *cosy.Ctx[model.User]) {
	value, exists := ctx.Payload["mfa_required"]
	if !exists {
		return
	}
	previous := false
	if ctx.ID != 0 {
		var u model.User
		if err := model.UseDB().First(&u, ctx.ID).Error; err != nil {
			ctx.AbortWithError(err)
			return
		}
		previous = u.MFARequired
	}
	if cast.ToBool(value) != previous {
		if !middleware.RequireMFAManagement(ctx.Context) {
			ctx.Abort()
		}
	}
}

func ResetUserMFA(c *gin.Context) {
	if !middleware.RequireMFAManagement(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
		return
	}
	if err := user.ResetMFA(id); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MFA credentials and sessions reset"})
}
