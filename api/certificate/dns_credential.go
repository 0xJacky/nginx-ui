package certificate

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"net/http"
)

func GetDnsCredential(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	d := query.DnsCredential

	dnsCredential, err := d.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	type apiDnsCredential struct {
		model.Model
		Name     string `json:"name"`
		Provider string `json:"provider"`
		dns.Config
	}
	c.JSON(http.StatusOK, apiDnsCredential{
		Model:    dnsCredential.Model,
		Name:     dnsCredential.Name,
		Provider: dnsCredential.Provider,
		Config:   *dnsCredential.Config,
	})
}

func GetDnsCredentialList(c *gin.Context) {
	cosy.Core[model.DnsCredential](c).SetFussy("provider").PagingList()
}

type DnsCredentialManageJson struct {
	Name     string `json:"name" binding:"required"`
	Provider string `json:"provider"`
	dns.Config
}

func AddDnsCredential(c *gin.Context) {
	var json DnsCredentialManageJson
	if !cosy.BindAndValid(c, &json) {
		return
	}

	json.Config.Name = json.Provider
	dnsCredential := model.DnsCredential{
		Name:     json.Name,
		Config:   &json.Config,
		Provider: json.Provider,
	}

	d := query.DnsCredential

	err := d.Create(&dnsCredential)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, dnsCredential)
}

func EditDnsCredential(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var json DnsCredentialManageJson
	if !cosy.BindAndValid(c, &json) {
		return
	}

	d := query.DnsCredential

	dnsCredential, err := d.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	json.Config.Name = json.Provider
	_, err = d.Where(d.ID.Eq(dnsCredential.ID)).Updates(&model.DnsCredential{
		Name:     json.Name,
		Config:   &json.Config,
		Provider: json.Provider,
	})

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	GetDnsCredential(c)
}

func DeleteDnsCredential(c *gin.Context) {
	cosy.Core[model.DnsCredential](c).Destroy()
}
