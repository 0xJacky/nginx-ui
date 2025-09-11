package audit

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

func LoggingMiddleware() gin.HandlerFunc {
	return logger.AuditMiddleware(func(c *gin.Context, logMap map[string]string) {
		var userId uint64
		if user, ok := c.Get("user"); ok {
			userId = user.(*model.User).ID
		}
		logMap["user_id"] = cast.ToString(userId)
	})
}
