package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/uozi-tech/cosy"
	"net/http"
)

func AddDomainToAutoCert(c *gin.Context) {
	name := c.Param("name")

	var json struct {
		DnsCredentialID uint64             `json:"dns_credential_id"`
		ChallengeMethod string             `json:"challenge_method"`
		Domains         []string           `json:"domains"`
		KeyType         certcrypto.KeyType `json:"key_type"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	certModel, err := model.FirstOrCreateCert(name, helper.GetKeyType(json.KeyType))

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
