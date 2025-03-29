package system

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
)

// System startup time
var startupTime time.Time

func init() {
	// Record system startup time
	startupTime = time.Now()
}

func installLockStatus() bool {
	return settings.NodeSettings.SkipInstallation || "" != cSettings.AppSettings.JwtSecret
}

// Check if installation time limit (10 minutes) is exceeded
func isInstallTimeoutExceeded() bool {
	return time.Since(startupTime) > 10*time.Minute
}

func InstallLockCheck(c *gin.Context) {
	locked := installLockStatus()
	timeout := false

	if !locked {
		timeout = isInstallTimeoutExceeded()
	}

	c.JSON(http.StatusOK, gin.H{
		"lock":    locked,
		"timeout": timeout,
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

	// Check if installation time limit (10 minutes) is exceeded
	if isInstallTimeoutExceeded() {
		cosy.ErrHandler(c, system.ErrInstallTimeout)
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
		cosy.ErrHandler(c, err)
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
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
