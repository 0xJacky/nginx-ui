package middleware

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

func Proxy() gin.HandlerFunc {
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

		env := query.Environment

		environment, err := env.Where(env.ID.Eq(id)).First()
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": err.Error(),
			})
			return
		}

		baseUrl, err := url.Parse(environment.URL)
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(baseUrl)

		customTransport, err := transport.NewTransport()
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		proxy.Transport = customTransport

		defaultDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			defaultDirector(req)
			req.Header.Del("X-Node-ID")
			req.Header.Set("X-Node-Secret", environment.Token)
		}

		// fix https://github.com/0xJacky/nginx-ui/issues/342
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode == http.StatusForbidden {
				resp.StatusCode = http.StatusServiceUnavailable
			}
			return nil
		}

		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		}

		logger.Debug("Proxy request", baseUrl.String()+c.Request.RequestURI)

		// fix proxy panic when client disconnect
		ctx := context.WithValue(
			c.Request.Context(),
			http.ServerContextKey,
			nil,
		)
		req := c.Request.Clone(ctx)

		proxy.ServeHTTP(c.Writer, req)
	}
}
