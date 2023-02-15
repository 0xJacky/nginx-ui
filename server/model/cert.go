package model

import (
    "github.com/0xJacky/Nginx-UI/server/pkg/nginx"
    "github.com/lib/pq"
    "os"
)

const (
    AutoCertEnabled  = 1
    AutoCertDisabled = -1
)

type CertDomains []string

type Cert struct {
    Model
    Name                  string         `json:"name"`
    Domains               pq.StringArray `json:"domains" gorm:"type:text[]"`
    Filename              string         `json:"filename"`
    SSLCertificatePath    string         `json:"ssl_certificate_path"`
    SSLCertificateKeyPath string         `json:"ssl_certificate_key_path"`
    AutoCert              int            `json:"auto_cert"`
    Log                   string         `json:"log"`
}

func FirstCert(confName string) (c Cert, err error) {
    err = db.First(&c, &Cert{
        Filename: confName,
    }).Error

    return
}

func FirstOrCreateCert(confName string) (c Cert, err error) {
    err = db.FirstOrCreate(&c, &Cert{Filename: confName}).Error
    return
}

func (c *Cert) Insert() error {
    return db.Create(c).Error
}

func GetAutoCertList() (c []*Cert) {
    var t []*Cert
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
        if enabledConfigMap[v.Filename] == true {
            c = append(c, v)
        }
    }

    return
}

func GetCertList(name, domain string) (c []Cert) {
    tx := db
    if name != "" {
        tx = tx.Where("name LIKE ? or domain LIKE ?", "%"+name+"%", "%"+name+"%")
    }
    if domain != "" {
        tx = tx.Where("domain LIKE ?", "%"+domain+"%")
    }
    tx.Find(&c)
    return
}

func FirstCertByID(id int) (c Cert, err error) {
    err = db.First(&c, id).Error

    return
}

func (c *Cert) Updates(n *Cert) error {
    return db.Model(&Cert{}).Where("id", c.ID).Updates(n).Error
}

func (c *Cert) Remove() error {
    if c.Filename == "" {
        return db.Delete(c).Error
    }

    return db.Where("filename", c.Filename).Delete(c).Error
}
