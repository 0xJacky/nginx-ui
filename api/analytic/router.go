package analytic

import (
	"github.com/gin-gonic/gin"
)

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("analytic", Analytic)
	r.GET("analytic/intro", GetNodeStat)
	r.GET("analytic/nodes", GetNodesAnalytic)
}

func InitRouter(r *gin.RouterGroup) {
	r.GET("analytic/init", GetAnalyticInit)
	r.GET("node", GetNode)
}
