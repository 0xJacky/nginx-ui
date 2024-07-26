package middleware

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/pretty66/websocketproxy"
	"github.com/spf13/cast"
	"net/http"
)

func ProxyWs() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID, ok := c.Get("ProxyNodeID")
		if !ok {
			c.Next()
			return
		}
		id := cast.ToInt(nodeID)
		if id == 0 {
			c.Next()
			return
		}

		defer c.Abort()

		env := query.Environment
		environment, err := env.Where(env.ID.Eq(id)).First()

		if err != nil {
			logger.Error(err)
			return
		}

		decodedUri, err := environment.GetWebSocketURL(c.Request.RequestURI)

		if err != nil {
			logger.Error(err)
			return
		}

		logger.Debug("Proxy request", decodedUri)

		wp, err := websocketproxy.NewProxy(decodedUri, func(r *http.Request) error {
			r.Header.Set("X-Node-Secret", environment.Token)
			return nil
		})

		if err != nil {
			logger.Error(err)
			return
		}

		wp.Proxy(c.Writer, c.Request)
	}
}
