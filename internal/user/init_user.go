package user

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// GetInitUser get the init user from database with caching
func GetInitUser(c *gin.Context) *model.User {
	const initUserID = 1
	// Try to get from cache first
	if cachedUser, found := GetCachedUser(initUserID); found {
		return cachedUser
	}
	
	// If not in cache, get from database
	db := cosy.UseDB(c)
	user := &model.User{}
	db.First(user, initUserID)
	
	// Cache the user for future requests
	if user.ID != 0 {
		CacheUser(user)
	}
	
	return user
}
