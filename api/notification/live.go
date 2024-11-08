package notification

import (
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"io"
	"time"
)

func Live(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// https://stackoverflow.com/questions/27898622/server-sent-events-stopped-work-after-enabling-ssl-on-proxy/27960243#27960243
	c.Header("X-Accel-Buffering", "no")

	evtChan := make(chan *model.Notification)

	notification.SetClient(c, evtChan)

	notify := c.Writer.CloseNotify()
	go func() {
		<-notify
		notification.RemoveClient(c)
	}()

	c.Stream(func(w io.Writer) bool {
		c.SSEvent("heartbeat", "")
		return false
	})

	timeout := time.NewTicker(30 * time.Second)

	for {
		select {
		case n := <-evtChan:
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("message", n)
				return false
			})
		case <-timeout.C:
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("heartbeat", "")
				return false
			})
		}
	}
}
