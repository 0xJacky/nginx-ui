package model

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"gorm.io/gorm/clause"
)

const (
	AutoCertSync              = 2
	AutoCertEnabled           = 1
	AutoCertDisabled          = -1
	CertChallengeMethodHTTP01 = "http01"
	CertChallengeMethodDNS01  = "dns01"
)

type CertDomains []string

type CertificateResource struct {
	*certificate.Resource
	PrivateKey        []byte `json:"private_key"`
	Certificate       []byte `json:"certificate"`
	IssuerCertificate []byte `json:"issuerCertificate"`
	CSR               []byte `json:"csr"`
}

type Cert struct {
	Model
	Name                    string               `json:"name"`
	Domains                 []string             `json:"domains" gorm:"serializer:json"`
	Filename                string               `json:"filename"`
	SSLCertificatePath      string               `json:"ssl_certificate_path"`
	SSLCertificateKeyPath   string               `json:"ssl_certificate_key_path"`
	AutoCert                int                  `json:"auto_cert"`
	ChallengeMethod         string               `json:"challenge_method"`
	DnsCredentialID         uint64               `json:"dns_credential_id"`
	DnsCredential           *DnsCredential       `json:"dns_credential,omitempty"`
	ACMEUserID              uint64               `json:"acme_user_id"`
	ACMEUser                *AcmeUser            `json:"acme_user,omitempty"`
	KeyType                 certcrypto.KeyType   `json:"key_type"`
	Log                     string               `json:"log"`
	Resource                *CertificateResource `json:"-" gorm:"serializer:json"`
	SyncNodeIds             []uint64             `json:"sync_node_ids" gorm:"serializer:json"`
	MustStaple              bool                 `json:"must_staple"`
	LegoDisableCNAMESupport bool                 `json:"lego_disable_cname_support"`
	RevokeOld               bool                 `json:"revoke_old"`
}

func FirstCert(confName string) (c Cert, err error) {
	err = db.Limit(1).Where(&Cert{
		Filename: confName,
	}).Find(&c).Error

	return
}

func FirstOrCreateCert(confName string, keyType certcrypto.KeyType) (c Cert, err error) {
	// Filename is used to check whether this site is enabled
	err = db.FirstOrCreate(&c, &Cert{Name: confName, Filename: confName, KeyType: keyType}).Error
	return
}

func FirstOrInit(confName string, keyType certcrypto.KeyType) (c Cert, err error) {
	err = db.FirstOrInit(&c, &Cert{Name: confName, Filename: confName, KeyType: keyType}).Error
	return
}

func (c *Cert) Insert() error {
	return db.Create(c).Error
}

func GetAutoCertList() (c []*Cert) {
	var t []*Cert
	if db == nil {
		return
	}
	db.Where("auto_cert", AutoCertEnabled).Find(&t)

	// check if this domain is enabled
	enabledConfig, err := os.ReadDir(nginx.GetConfPath("sites-enabled"))

	if err != nil {
		return
	}

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	for _, v := range t {
		if v.ChallengeMethod == CertChallengeMethodDNS01 || enabledConfigMap[v.Filename] == true {
			c = append(c, v)
		}
	}

	return
}

func (c *Cert) Updates(n *Cert) error {
	return db.Model(c).Clauses(clause.Returning{}).
		Where("id", c.ID).Updates(n).Error
}

func (c *Cert) Remove() error {
	if c.Filename == "" {
		return db.Delete(c).Error
	}

	return db.Where("filename", c.Filename).Delete(c).Error
}

func (c *Cert) GetKeyType() certcrypto.KeyType {
	return helper.GetKeyType(c.KeyType)
}

func (c *CertificateResource) GetResource() certificate.Resource {
	return certificate.Resource{
		Domain:            c.Resource.Domain,
		CertURL:           c.Resource.CertURL,
		CertStableURL:     c.Resource.CertStableURL,
		PrivateKey:        c.PrivateKey,
		Certificate:       c.Certificate,
		IssuerCertificate: c.IssuerCertificate,
		CSR:               c.CSR,
	}
}

// GetCertList returns all certificates
func GetCertList() (c []*Cert) {
	if db == nil {
		return
	}
	db.Find(&c)
	return
}
