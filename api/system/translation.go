package system

import (
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetTranslation(c *gin.Context) {
	code := c.Param("code")

	c.JSON(http.StatusOK, translation.GetTranslation(code))
}
