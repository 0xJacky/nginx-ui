package api

import (
    "github.com/0xJacky/Nginx-UI/frontend"
    "github.com/gin-gonic/gin"
    "net/http"
)

func GetTranslations(c *gin.Context) {
    c.JSON(http.StatusOK, frontend.Translations)
}
