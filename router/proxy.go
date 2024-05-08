package router

import (
	"crypto/tls"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/query"
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

		baseUrl, err := url.Parse(environment.URL)
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		proxyUrl, err := baseUrl.Parse(c.Request.RequestURI)
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		logger.Debug("Proxy request", proxyUrl.String())
		client := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		req, err := http.NewRequest(c.Request.Method, proxyUrl.String(), c.Request.Body)
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

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

		// rewrite status code to fix https://github.com/0xJacky/nginx-ui/issues/342
		if resp.StatusCode == http.StatusForbidden {
			resp.StatusCode = http.StatusServiceUnavailable
		}

		c.Writer.WriteHeader(resp.StatusCode)

		c.Writer.Header().Add("Content-Type", resp.Header.Get("Content-Type"))

		_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			logger.Error(err)
			return
		}
	}
}
