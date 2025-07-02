package user

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// GetInitUser get the init user from database
func GetInitUser(c *gin.Context) *model.User {
	db := cosy.UseDB(c)
	user := &model.User{}
	db.First(user, 1)
	return user
}
