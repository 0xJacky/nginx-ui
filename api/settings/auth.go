package settings

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func GetBanLoginIP(c *gin.Context) {
	b := query.BanIP

	// clear expired banned IPs
	_, _ = b.Where(b.ExpiredAt.Lte(time.Now().Unix())).Delete()

	banIps, err := b.Where(
		b.ExpiredAt.Gte(time.Now().Unix()),
		b.Attempts.Gte(settings.AuthSettings.MaxAttempts)).Find()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, banIps)
}

func RemoveBannedIP(c *gin.Context) {
	var json struct {
		IP string `json:"ip"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}

	b := query.BanIP
	_, err := b.Where(b.IP.Eq(json.IP)).Delete()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
