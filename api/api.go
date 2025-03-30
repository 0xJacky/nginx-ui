package api

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
)

func CurrentUser(c *gin.Context) *model.User {
	return c.MustGet("user").(*model.User)
}

func SetSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// https://stackoverflow.com/questions/27898622/server-sent-events-stopped-work-after-enabling-ssl-on-proxy/27960243#27960243
	c.Header("X-Accel-Buffering", "no")
}
