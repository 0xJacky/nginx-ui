package certificate

import (
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

type redactedDnsCredential struct {
	model.Model
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	ProviderCode string `json:"provider_code"`
}

func redactDnsCredential(credential *model.DnsCredential) redactedDnsCredential {
	return redactedDnsCredential{
		Model:        credential.Model,
		Name:         credential.Name,
		Provider:     credential.Provider,
		ProviderCode: credential.ProviderCode,
	}
}

func GetDnsCredential(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	d := query.DnsCredential

	dnsCredential, err := d.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if internalmcp.IsServiceTokenRequest(c) {
		c.JSON(http.StatusOK, redactDnsCredential(dnsCredential))
		return
	}
	type apiDnsCredential struct {
		model.Model
		Name         string `json:"name"`
		Provider     string `json:"provider"`
		ProviderCode string `json:"provider_code"`
		dns.Config
	}
	c.JSON(http.StatusOK, apiDnsCredential{
		Model:        dnsCredential.Model,
		Name:         dnsCredential.Name,
		Provider:     dnsCredential.Provider,
		ProviderCode: dnsCredential.ProviderCode,
		Config:       *dnsCredential.Config,
	})
}

func GetDnsCredentialList(c *gin.Context) {
	core := cosy.Core[model.DnsCredential](c).
		SetEqual("provider_code").
		SetEqual("provider").
		SetFussy("name")
	if internalmcp.IsServiceTokenRequest(c) {
		core.SetTransformer(redactDnsCredential)
	}
	core.PagingList()
}

type DnsCredentialManageJson struct {
	Name         string `json:"name" binding:"required"`
	Provider     string `json:"provider"`
	ProviderCode string `json:"provider_code"`
	dns.Config
}

func AddDnsCredential(c *gin.Context) {
	var json DnsCredentialManageJson
	if !cosy.BindAndValid(c, &json) {
		return
	}

	providerCode := resolveProviderCode(json)
	json.Config.Code = providerCode
	json.Config.Name = json.Provider
	dnsCredential := model.DnsCredential{
		Name:         json.Name,
		Config:       &json.Config,
		Provider:     json.Provider,
		ProviderCode: providerCode,
	}

	d := query.DnsCredential

	err := d.Create(&dnsCredential)
	if err != nil {
		cosy.ErrHandler(c, err)
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
		cosy.ErrHandler(c, err)
		return
	}

	json.Config.Name = json.Provider
	json.Config.Code = resolveProviderCode(json)
	_, err = d.Where(d.ID.Eq(dnsCredential.ID)).Updates(&model.DnsCredential{
		Name:         json.Name,
		Config:       &json.Config,
		Provider:     json.Provider,
		ProviderCode: resolveProviderCode(json),
	})

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	GetDnsCredential(c)
}

func DeleteDnsCredential(c *gin.Context) {
	cosy.Core[model.DnsCredential](c).Destroy()
}

func resolveProviderCode(payload DnsCredentialManageJson) string {
	if trimmed := normalizeProviderCode(payload.ProviderCode); trimmed != "" {
		return trimmed
	}
	if trimmed := normalizeProviderCode(payload.Code); trimmed != "" {
		return trimmed
	}
	return normalizeProviderCode(payload.Provider)
}

func normalizeProviderCode(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
