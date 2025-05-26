package user

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"golang.org/x/crypto/bcrypt"
)

func GetCurrentUser(c *gin.Context) {
	user := api.CurrentUser(c)
	c.JSON(http.StatusOK, user)
}

func UpdateCurrentUser(c *gin.Context) {
	cosy.Core[model.User](c).
		SetValidRules(gin.H{
			"name": "required",
		}).
		Custom(func(c *cosy.Ctx[model.User]) {
			user := api.CurrentUser(c.Context)
			user.Name = c.Model.Name

			db := cosy.UseDB()
			err := db.Where("id = ?", user.ID).Updates(user).Error
			if err != nil {
				cosy.ErrHandler(c.Context, err)
				return
			}

			c.JSON(http.StatusOK, user)
		})
}

func UpdateCurrentUserPassword(c *gin.Context) {
	var json struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	user := api.CurrentUser(c)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(json.OldPassword)); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	user.Password = json.NewPassword

	pwdBytes, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	db := cosy.UseDB()
	err = db.Where("id = ?", user.ID).Updates(&model.User{
		Password: string(pwdBytes),
	}).Error
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
