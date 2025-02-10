package upstream

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("/availability_test", AvailabilityTest)
}
