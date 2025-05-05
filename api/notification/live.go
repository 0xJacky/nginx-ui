package notification

import (
	"io"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
)

func Live(c *gin.Context) {
	api.SetSSEHeaders(c)

	evtChan := make(chan *model.Notification)

	notification.SetClient(c, evtChan)

	notify := c.Writer.CloseNotify()

	c.Stream(func(w io.Writer) bool {
		c.SSEvent("heartbeat", "")
		return false
	})

	for {
		select {
		case n := <-evtChan:
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("message", n)
				return false
			})
		case <-time.After(30 * time.Second):
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("heartbeat", "")
				return false
			})
		case <-notify:
			notification.RemoveClient(c)
			return
		case <-kernel.Context.Done():
			return
		}
	}
}
