package user

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

func GetUsers(c *gin.Context) {
	cosy.Core[model.User](c).SetFussy("name").PagingList()
}

func GetUser(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	u := query.User

	user, err := u.FirstByID(id)

	if err != nil {
		api.ErrHandler(c, err)
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
	ok := api.BindAndValid(c, &json)
	if !ok {
		return
	}

	u := query.User

	pwd, err := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	json.Password = string(pwd)

	user := model.User{
		Name:     json.Name,
		Password: json.Password,
	}

	err = u.Create(&user)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, user)

}

func EditUser(c *gin.Context) {
	userId := cast.ToUint64(c.Param("id"))

	if settings.NodeSettings.Demo && userId == 1 {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Changing user password is forbidden in demo mode",
		})
		return
	}

	var json UserJson
	ok := api.BindAndValid(c, &json)
	if !ok {
		return
	}

	u := query.User
	user, err := u.FirstByID(userId)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	edit := &model.User{
		Name: json.Name,
	}

	// encrypt password
	if json.Password != "" {
		var pwd []byte
		pwd, err = bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
		edit.Password = string(pwd)
	}

	_, err = u.Where(u.ID.Eq(userId)).Updates(&edit)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func DeleteUser(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))
	if cast.ToInt(id) == 1 {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Prohibit deleting the default user",
		})
		return
	}
	cosy.Core[model.User](c).Destroy()
}

func RecoverUser(c *gin.Context) {
	cosy.Core[model.User](c).Recover()
}
