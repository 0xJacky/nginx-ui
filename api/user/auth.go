package user

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginUser struct {
	Name     string `json:"name" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=255"`
}

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

func Login(c *gin.Context) {
	var user LoginUser
	ok := api.BindAndValid(c, &user)
	if !ok {
		return
	}

	u, _ := model.GetUser(user.Name)

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(user.Password)); err != nil {
		time.Sleep(5 * time.Second)
		c.JSON(http.StatusForbidden, gin.H{
			"message": "The username or password is incorrect",
		})
		return
	}

	token, err := model.GenerateJWT(u.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Message: "ok",
		Token:   token,
	})
}

func Logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token != "" {
		err := model.DeleteToken(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusNoContent, nil)
}
