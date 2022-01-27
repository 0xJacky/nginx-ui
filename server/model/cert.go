package model

type Cert struct {
	Model
	Domain string `json:"domain"`
}

func FirstCert(domain string) (c Cert, err error) {
	err = db.First(&c, &Cert{
		Domain: domain,
	}).Error

	return
}

func FirstOrCreateCert(domain string) (c Cert, err error) {
	err = db.FirstOrCreate(&c, &Cert{Domain: domain}).Error
	return
}

func GetAutoCertList() (c []Cert) {
	db.Find(&c)
	return
}

func (c *Cert) Remove() error {
	return db.Where("domain", c.Domain).Delete(c).Error
}
