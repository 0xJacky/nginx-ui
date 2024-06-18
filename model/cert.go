package model

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/lib/pq"
	"gorm.io/gorm/clause"
	"os"
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
	Name                  string               `json:"name"`
	Domains               pq.StringArray       `json:"domains" gorm:"type:text[]"`
	Filename              string               `json:"filename"`
	SSLCertificatePath    string               `json:"ssl_certificate_path"`
	SSLCertificateKeyPath string               `json:"ssl_certificate_key_path"`
	AutoCert              int                  `json:"auto_cert"`
	ChallengeMethod       string               `json:"challenge_method"`
	DnsCredentialID       int                  `json:"dns_credential_id"`
	DnsCredential         *DnsCredential       `json:"dns_credential,omitempty"`
	ACMEUserID            int                  `json:"acme_user_id"`
	ACMEUser              *AcmeUser            `json:"acme_user,omitempty"`
	KeyType               certcrypto.KeyType   `json:"key_type"`
	Log                   string               `json:"log"`
	Resource              *CertificateResource `json:"-" gorm:"serializer:json"`
	SyncNodeIds           []int                `json:"sync_node_ids" gorm:"serializer:json"`
}

func FirstCert(confName string) (c Cert, err error) {
	err = db.First(&c, &Cert{
		Filename: confName,
	}).Error

	return
}

func FirstOrCreateCert(confName string, keyType certcrypto.KeyType) (c Cert, err error) {
	// Filename is used to check whether this site is enabled
	err = db.FirstOrCreate(&c, &Cert{Name: confName, Filename: confName, KeyType: keyType}).Error
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
