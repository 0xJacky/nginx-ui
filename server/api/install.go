package api

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

func installLockStatus() bool {
	return "" != settings.ServerSettings.JwtSecret
}

func InstallLockCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"lock": installLockStatus(),
	})
}

type InstallJson struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=255"`
	Database string `json:"database"`
}

func InstallNginxUI(c *gin.Context) {
	// Visit this api after installed is forbidden
	if installLockStatus() {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "installed",
		})
		return
	}
	var json InstallJson
	ok := BindAndValid(c, &json)
	if !ok {
		return
	}

	settings.ServerSettings.JwtSecret = uuid.New().String()
	settings.ServerSettings.Email = json.Email
	if "" != json.Database {
		settings.ServerSettings.Database = json.Database
	}
	settings.ReflectFrom()

	err := settings.Save()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	// Init model
	db := model.Init()
	query.Init(db)

	curd := model.NewCurd(&model.Auth{})
	pwd, _ := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
	err = curd.Add(&model.Auth{
		Name:     json.Username,
		Password: string(pwd),
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
