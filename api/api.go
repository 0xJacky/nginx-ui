package api

import (
	"errors"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
	"net/http"
)

func CurrentUser(c *gin.Context) *model.User {
	return c.MustGet("user").(*model.User)
}

func ErrHandler(c *gin.Context, err error) {
	logger.GetLogger().Errorln(err)
	var cErr *cosy.Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, &cosy.Error{
			Code:    http.StatusNotFound,
			Message: gorm.ErrRecordNotFound.Error(),
		})
	case errors.As(err, &cErr):
		c.JSON(http.StatusInternalServerError, cErr)
	default:
		c.JSON(http.StatusInternalServerError, &cosy.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}
}

func SetSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// https://stackoverflow.com/questions/27898622/server-sent-events-stopped-work-after-enabling-ssl-on-proxy/27960243#27960243
	c.Header("X-Accel-Buffering", "no")
}
