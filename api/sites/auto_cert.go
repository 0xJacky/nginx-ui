package sites

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm/clause"
)

type autoCertRequest struct {
	DnsCredentialID         uint64             `json:"dns_credential_id"`
	ChallengeMethod         string             `json:"challenge_method"`
	Profile                 string             `json:"profile"`
	Domains                 []string           `json:"domains"`
	KeyType                 certcrypto.KeyType `json:"key_type"`
	ACMEUserID              uint64             `json:"acme_user_id"`
	MustStaple              bool               `json:"must_staple"`
	LegoDisableCNAMESupport bool               `json:"lego_disable_cname_support"`
	EnableCommonName        bool               `json:"enable_common_name"`
	RevokeOld               bool               `json:"revoke_old"`
}

func AddDomainToAutoCert(c *gin.Context) {
	name := c.Param("name")

	var json autoCertRequest

	if !cosy.BindAndValid(c, &json) {
		return
	}

	payload := &cert.ConfigPayload{
		ServerName:      json.Domains,
		ChallengeMethod: json.ChallengeMethod,
	}
	if err := cert.NormalizeAndValidateIdentifiers(payload); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	json.Domains = payload.ServerName
	json.ChallengeMethod = payload.ChallengeMethod

	certModel, err := model.FirstOrCreateCert(name, helper.GetKeyType(json.KeyType))

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	err = persistAutoCertOptions(&certModel, name, json)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, certModel)
}

func RemoveDomainFromAutoCert(c *gin.Context) {
	name := c.Param("name")
	certModel, err := model.FirstCert(name)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		AutoCert: model.AutoCertDisabled,
	})

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, nil)
}

func persistAutoCertOptions(certModel *model.Cert, name string, json autoCertRequest) error {
	profile := json.Profile
	if profile == "" {
		profile = certModel.Profile
		if profile == "" && certModel.Resource != nil && certModel.Resource.Resource != nil {
			profile = certModel.Resource.Profile
		}
	}

	updates := &model.Cert{
		Name:                    name,
		Domains:                 json.Domains,
		AutoCert:                model.AutoCertEnabled,
		DnsCredentialID:         json.DnsCredentialID,
		ChallengeMethod:         json.ChallengeMethod,
		Profile:                 profile,
		KeyType:                 helper.GetKeyType(json.KeyType),
		ACMEUserID:              json.ACMEUserID,
		MustStaple:              json.MustStaple,
		LegoDisableCNAMESupport: json.LegoDisableCNAMESupport,
		EnableCommonName:        json.EnableCommonName,
		RevokeOld:               json.RevokeOld,
	}

	return model.UseDB().Model(certModel).Clauses(clause.Returning{}).
		Select(
			"name", "domains", "auto_cert", "dns_credential_id", "challenge_method", "profile",
			"key_type", "acme_user_id", "must_staple", "lego_disable_cname_support",
			"enable_common_name", "revoke_old",
		).
		Updates(updates).Error
}
