package external_notify

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func InitRouter(r *gin.RouterGroup) {
	c := cosy.Api[model.ExternalNotify]("/external_notifies")

	c.InitRouter(r)
}
