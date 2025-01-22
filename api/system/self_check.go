package system

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/self_check"
	"github.com/gin-gonic/gin"
)

func SelfCheck(c *gin.Context) {
	report := self_check.Run()
	c.JSON(http.StatusOK, report)
}

func SelfCheckFix(c *gin.Context) {
	result := self_check.AttemptFix(c.Param("name"))
	c.JSON(http.StatusOK, result)
}
