package certificate

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	model2 "github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
)

func GetDnsCredential(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	d := query.DnsCredential

	dnsCredential, err := d.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	type apiDnsCredential struct {
		model2.Model
		Name string `json:"name"`
		dns.Config
	}
	c.JSON(http.StatusOK, apiDnsCredential{
		Model:  dnsCredential.Model,
		Name:   dnsCredential.Name,
		Config: *dnsCredential.Config,
	})
}

func GetDnsCredentialList(c *gin.Context) {
	d := query.DnsCredential
	provider := c.Query("provider")
	var data []*model2.DnsCredential
	var err error
	if provider != "" {
		data, err = d.Where(d.Provider.Eq(provider)).Find()
	} else {
		data, err = d.Find()
	}

	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

type DnsCredentialManageJson struct {
	Name     string `json:"name" binding:"required"`
	Provider string `json:"provider"`
	dns.Config
}

func AddDnsCredential(c *gin.Context) {
	var json DnsCredentialManageJson
	if !api.BindAndValid(c, &json) {
		return
	}

	json.Config.Name = json.Provider
	dnsCredential := model2.DnsCredential{
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
	id := cast.ToInt(c.Param("id"))

	var json DnsCredentialManageJson
	if !api.BindAndValid(c, &json) {
		return
	}

	d := query.DnsCredential

	dnsCredential, err := d.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	json.Config.Name = json.Provider
	_, err = d.Where(d.ID.Eq(dnsCredential.ID)).Updates(&model2.DnsCredential{
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
	id := cast.ToInt(c.Param("id"))
	d := query.DnsCredential

	dnsCredential, err := d.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	err = d.DeleteByID(dnsCredential.ID)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
