package system

import (
	"net/http"

	internalSystem "github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
)

func InstallLockCheck(c *gin.Context) {
	locked := internalSystem.InstallLockStatus()
	timeout := false

	if !locked {
		timeout = internalSystem.IsInstallTimeoutExceeded()
	}

	c.JSON(http.StatusOK, gin.H{
		"lock":    locked,
		"timeout": timeout,
	})
}

type InstallJson struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=20"`
}

func InstallNginxUI(c *gin.Context) {
	// Visit this api after installed is forbidden
	if internalSystem.InstallLockStatus() {
		cosy.ErrHandler(c, internalSystem.ErrInstalled)
		return
	}

	// Check if installation time limit (10 minutes) is exceeded
	if internalSystem.IsInstallTimeoutExceeded() {
		cosy.ErrHandler(c, internalSystem.ErrInstallTimeout)
		return
	}

	var json InstallJson
	ok := cosy.BindAndValid(c, &json)
	if !ok {
		return
	}

	err := settings.Update(func() {
		cSettings.AppSettings.JwtSecret = uuid.New().String()
		settings.NodeSettings.Secret = uuid.New().String()
		settings.CertSettings.Email = json.Email
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	pwd, err := bcrypt.GenerateFromPassword([]byte(json.Password), bcrypt.DefaultCost)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	u := query.User
	_, err = u.Where(u.ID.Eq(1)).Updates(&model.User{
		Name:     json.Username,
		Password: string(pwd),
	})

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err := internalSystem.ConsumeInstallSecret(); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
