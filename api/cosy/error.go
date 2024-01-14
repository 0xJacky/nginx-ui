package cosy

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func errHandler(c *gin.Context, err error) {
	logger.GetLogger().WithOptions(zap.AddCallerSkip(1)).Errorln(err)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": err.Error(),
	})
}
