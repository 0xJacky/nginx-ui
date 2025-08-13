package middleware

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/pretty66/websocketproxy"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

func ProxyWs() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID, ok := c.Get("ProxyNodeID")
		if !ok {
			c.Next()
			return
		}
		id := cast.ToUint64(nodeID)
		if id == 0 {
			c.Next()
			return
		}

		defer c.Abort()

		nodeQuery := query.Node
		node, err := nodeQuery.Where(nodeQuery.ID.Eq(id)).First()
		if err != nil {
			logger.Error(err)
			return
		}

		decodedUri, err := node.GetWebSocketURL(c.Request.RequestURI)

		if err != nil {
			logger.Error(err)
			return
		}

		logger.Debug("Proxy request", decodedUri)

		wp, err := websocketproxy.NewProxy(decodedUri, func(r *http.Request) error {
			r.Header.Set("X-Node-Secret", node.Token)
			r.Header.Del("X-Node-ID")
			queryValues := r.URL.Query()
			queryValues.Del("x_node_id")
			r.URL.RawQuery = queryValues.Encode()
			return nil
		})

		if err != nil {
			logger.Error(err)
			return
		}

		wp.Proxy(c.Writer, c.Request)
	}
}
