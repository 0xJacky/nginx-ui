package settings

import "github.com/go-acme/lego/v5/lego"

type Cert struct {
	Email                string   `json:"email" protected:"true"`
	CADir                string   `json:"ca_dir" binding:"omitempty,url"`
	RenewalInterval      int      `json:"renewal_interval" binding:"min=1,max=90"`
	RecursiveNameservers []string `json:"recursive_nameservers" binding:"omitempty,dive,hostname_port"`
	HTTPChallengePort    string   `json:"http_challenge_port"`
}

var CertSettings = &Cert{
	Email:                "",
	CADir:                "",
	RenewalInterval:      30,
	RecursiveNameservers: []string{},
	HTTPChallengePort:    "9180",
}

func (s *Cert) GetCADir() string {
	if s.CADir != "" {
		return s.CADir
	}
	return lego.DirectoryURLLetsEncrypt
}

// GetCertRenewalInterval returns the configured remaining-validity threshold in days.
func (s *Cert) GetCertRenewalInterval() int {
	if s.RenewalInterval < 1 {
		return 1
	}
	if s.RenewalInterval > 90 {
		return 90
	}
	return s.RenewalInterval
}
