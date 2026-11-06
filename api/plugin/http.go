package plugin

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/capability"
	"github.com/gin-gonic/gin"
)

// InitHTTPRouter registers the http capability route. It is kept separate
// from InitRouter because it is not a management endpoint: it forwards
// arbitrary requests to a plugin and streams the reply back.
//
// It takes the /api group itself rather than the authenticated one because a
// browser WebSocket cannot send an Authorization header: upgrade requests
// authenticate and proxy like the host's own WebSocket routes (credentials in
// the query string), every other request uses the header based chain.
func InitHTTPRouter(r *gin.RouterGroup) {
	r.Any("/plugins/:id/http/*path",
		middleware.ForWebSocketUpgrade(middleware.RequireWebSocketOrigin(), func(c *gin.Context) { c.Next() }),
		middleware.ForWebSocketUpgrade(middleware.AuthRequiredWS(), middleware.AuthRequired()),
		middleware.ForWebSocketUpgrade(middleware.ProxyWs(), middleware.Proxy()),
		capability.NewHTTPHandler(plugin.GetManager()),
	)
}
