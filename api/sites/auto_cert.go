package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AddDomainToAutoCert(c *gin.Context) {
	name := c.Param("name")

	var json struct {
		DnsCredentialID int      `json:"dns_credential_id"`
		ChallengeMethod string   `json:"challenge_method"`
		Domains         []string `json:"domains"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	certModel, err := model.FirstOrCreateCert(name)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:            name,
		Domains:         json.Domains,
		AutoCert:        model.AutoCertEnabled,
		DnsCredentialID: json.DnsCredentialID,
		ChallengeMethod: json.ChallengeMethod,
	})

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, certModel)
}

func RemoveDomainFromAutoCert(c *gin.Context) {
	name := c.Param("name")
	certModel, err := model.FirstCert(name)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		AutoCert: model.AutoCertDisabled,
	})

	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, nil)
}
