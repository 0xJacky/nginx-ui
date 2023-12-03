package certificate

import (
    "github.com/0xJacky/Nginx-UI/internal/cert/dns"
    "github.com/gin-gonic/gin"
    "net/http"
)

func GetDNSProvidersList(c *gin.Context) {
    c.JSON(http.StatusOK, dns.GetProvidersList())
}

func GetDNSProvider(c *gin.Context) {
    code := c.Param("code")

    provider, ok := dns.GetProvider(code)

    if !ok {
        c.JSON(http.StatusNotFound, gin.H{
            "message": "provider not found",
        })
        return
    }

    c.JSON(http.StatusOK, provider)
}

