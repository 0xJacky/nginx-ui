package config

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetBasePath(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"base_path": nginx.GetConfPath(),
	})
}
