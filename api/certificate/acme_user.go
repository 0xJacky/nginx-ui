package certificate

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cosy"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
)

func GetAcmeUser(c *gin.Context) {
	u := query.AcmeUser
	id := cast.ToInt(c.Param("id"))
	user, err := u.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func CreateAcmeUser(c *gin.Context) {
	cosy.Core[model.AcmeUser](c).SetValidRules(gin.H{
		"name":   "required",
		"email":  "required,email",
		"ca_dir": "omitempty",
	}).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AcmeUser]) {
		if ctx.Model.CADir == "" {
			ctx.Model.CADir = settings.ServerSettings.CADir
		}
		err := ctx.Model.Register()
		if err != nil {
			ctx.AbortWithError(err)
			return
		}
	}).Create()
}

func ModifyAcmeUser(c *gin.Context) {
	cosy.Core[model.AcmeUser](c).SetValidRules(gin.H{
		"name":   "omitempty",
		"email":  "omitempty,email",
		"ca_dir": "omitempty",
	}).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AcmeUser]) {
		if ctx.Model.CADir == "" {
			ctx.Model.CADir = settings.ServerSettings.CADir
		}

		if ctx.OriginModel.Email != ctx.Model.Email ||
			ctx.OriginModel.CADir != ctx.Model.CADir {
			err := ctx.Model.Register()
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
		}
	}).Modify()
}

func GetAcmeUserList(c *gin.Context) {
	cosy.Core[model.AcmeUser](c).
		SetFussy("name", "email").
		PagingList()
}

func DestroyAcmeUser(c *gin.Context) {
	cosy.Core[model.AcmeUser](c).Destroy()
}

func RecoverAcmeUser(c *gin.Context) {
	cosy.Core[model.AcmeUser](c).Recover()
}

func RegisterAcmeUser(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))
	u := query.AcmeUser
	user, err := u.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	err = user.Register()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	_, err = u.Where(u.ID.Eq(id)).Updates(user)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}
