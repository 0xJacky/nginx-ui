package user

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"net/http"
	"time"
)

type LoginUser struct {
	Name     string `json:"name" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=255"`
}

const (
	ErrPasswordIncorrect = 4031
	ErrMaxAttempts       = 4291
	ErrUserBanned        = 4033
)

type LoginResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Code    int    `json:"code"`
	Token   string `json:"token,omitempty"`
}

func Login(c *gin.Context) {
	var json LoginUser
	ok := api.BindAndValid(c, &json)
	if !ok {
		return
	}

	u, err := user.Login(json.Name, json.Password)
	if err != nil {
		time.Sleep(5 * time.Second)
		switch {
		case errors.Is(err, user.ErrPasswordIncorrect):
			c.JSON(http.StatusForbidden, LoginResponse{
				Message: "Password incorrect",
				Code:    ErrPasswordIncorrect,
			})
		case errors.Is(err, user.ErrUserBanned):
			c.JSON(http.StatusForbidden, LoginResponse{
				Message: "The user is banned",
				Code:    ErrUserBanned,
			})
		default:
			api.ErrHandler(c, err)
		}
		return
	}

	logger.Info("[User Login]", u.Name)
	token, err := user.GenerateJWT(u.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Message: err.Error(),
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
		err := user.DeleteToken(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusNoContent, nil)
}
