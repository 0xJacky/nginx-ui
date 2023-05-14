package router

import (
	"crypto/tls"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"io"
	"net/http"
	"net/url"
)

func proxy() gin.HandlerFunc {
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
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": err.Error(),
			})
			return
		}

		u, err := url.JoinPath(environment.URL, c.Request.RequestURI)

		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		decodedUri, err := url.QueryUnescape(u)

		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		logger.Debug("Proxy request", decodedUri)
		client := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		req, err := http.NewRequest(c.Request.Method, decodedUri, c.Request.Body)
		req.Header.Set("X-Node-Secret", environment.Token)

		resp, err := client.Do(req)

		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		defer resp.Body.Close()

		c.Writer.WriteHeader(resp.StatusCode)

		c.Writer.Header().Add("Content-Type", resp.Header.Get("Content-Type"))

		_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			logger.Error(err)
			return
		}
	}
}
