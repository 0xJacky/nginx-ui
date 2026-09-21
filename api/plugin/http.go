package plugin

import (
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/capability"
	"github.com/gin-gonic/gin"
)

// InitHTTPRouter registers the http capability route. It is kept separate
// from InitRouter because it is not a management endpoint: it forwards
// arbitrary requests to a plugin and streams the reply back.
func InitHTTPRouter(g *gin.RouterGroup) {
	g.Any("/plugins/:id/http/*path", capability.NewHTTPHandler(plugin.GetManager()))
}
