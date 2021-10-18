package api

import (
	"errors"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

func GetUsers(c *gin.Context) {
	curd := model.NewCurd(&model.Auth{})

	var list []model.Auth
	err := curd.GetList(&list)

	if err != nil {
		ErrorHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": list,
	})
}

func GetUser(c *gin.Context) {
	curd := model.NewCurd(&model.Auth{})
	id := c.Param("id")

	var user model.Auth
	err := curd.First(&user, id)

	if err != nil {
		ErrorHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

type UserJson struct {
	Name     string `json:"name" binding:"required,max=255"`
	Password string `json:"password" binding:"max=255"`
}

func AddUser(c *gin.Context) {
	var json UserJson
	ok := BindAndValid(c, &json)
	if !ok {
        return
	}
	curd := model.NewCurd(&model.Auth{})

    pwd, err := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
    if err != nil {
        ErrorHandler(c, err)
        return
    }
    json.Password = string(pwd)

	user := model.Auth{
		Name:     json.Name,
		Password: json.Password,
	}

	err = curd.Add(&user)

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, user)

}

func EditUser(c *gin.Context) {
	var json UserJson
	ok := BindAndValid(c, &json)
	if !ok {
        return
	}
	curd := model.NewCurd(&model.Auth{})

	var user, edit model.Auth

	err := curd.First(&user, c.Param("id"))

	if err != nil {
		ErrorHandler(c, err)
		return
	}
	edit.Name = json.Name

	// 改密码加密
	if json.Password != "" {
		var pwd []byte
		pwd, err = bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
		if err != nil {
			ErrorHandler(c, err)
			return
		}
		edit.Password = string(pwd)
	}

	err = curd.Edit(&user, &edit)

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")

	if cast.ToInt(id) == 1 {
		ErrorHandler(c, errors.New("不允许删除默认账户"))
		return
	}
	curd := model.NewCurd(&model.Auth{})
	err := curd.Delete(&model.Auth{}, "id", id)
	if err != nil {
		ErrorHandler(c, err)
		return
	}
	c.JSON(http.StatusNoContent, gin.H{})
}
