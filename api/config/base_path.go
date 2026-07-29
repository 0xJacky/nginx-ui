package config

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
)

func GetBasePath(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"base_path": nginx.GetConfPath(),
	})
}
