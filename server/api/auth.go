package api

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

type LoginUser struct {
	Name     string `json:"name" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=255"`
}

func Login(c *gin.Context) {
	var user LoginUser
	ok := BindAndValid(c, &user)
	if !ok {
		return
	}

	u, _ := model.GetUser(user.Name)

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(user.Password)); err != nil {
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

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"token":   token,
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
