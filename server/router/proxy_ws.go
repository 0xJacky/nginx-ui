package router

import (
    "github.com/0xJacky/Nginx-UI/server/internal/logger"
    "github.com/0xJacky/Nginx-UI/server/query"
    "github.com/gin-gonic/gin"
    "github.com/pretty66/websocketproxy"
    "github.com/spf13/cast"
    "net/http"
    "net/url"
    "strings"
)

func proxyWs() gin.HandlerFunc {
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

        baseUrl, err := url.Parse(environment.URL)
        if err != nil {
            logger.Error(err)
            return
        }

        logger.Debug(baseUrl.Port())
        defaultPort := ""
        if baseUrl.Port() == "" {
            switch baseUrl.Scheme {
            default:
                fallthrough
            case "http":
                defaultPort = "80"
            case "https":
                defaultPort = "443"
            }

            baseUrl.Host = baseUrl.Hostname() + ":" + defaultPort
        }
        logger.Debug(baseUrl.String())

        u, err := url.JoinPath(baseUrl.String(), c.Request.RequestURI)

        if err != nil {
            logger.Error(err)
            return
        }

        decodedUri, err := url.QueryUnescape(u)

        if err != nil {
            logger.Error(err)
            return
        }

        // http will be replaced with ws, https will be replaced with wss
        decodedUri = strings.ReplaceAll(decodedUri, "http", "ws")

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
