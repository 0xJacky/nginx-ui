package system

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

func installLockStatus() bool {
	return settings.NodeSettings.SkipInstallation || "" != cSettings.AppSettings.JwtSecret
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
	ok := cosy.BindAndValid(c, &json)
	if !ok {
		return
	}

	cSettings.AppSettings.JwtSecret = uuid.New().String()
	settings.NodeSettings.Secret = uuid.New().String()
	settings.CertSettings.Email = json.Email
	if "" != json.Database {
		settings.DatabaseSettings.Name = json.Database
	}

	err := settings.Save()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	// Init model
	kernel.InitDatabase()

	pwd, _ := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)

	u := query.User
	err = u.Create(&model.User{
		Name:     json.Username,
		Password: string(pwd),
	})

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
