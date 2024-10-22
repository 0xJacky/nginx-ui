package settings

import "github.com/go-acme/lego/v4/lego"

type Cert struct {
	Email                string   `json:"email" protected:"true"`
	CADir                string   `json:"ca_dir" binding:"omitempty,url"`
	RenewalInterval      int      `json:"cert_renewal_interval" binding:"min=7,max=21"`
	RecursiveNameservers []string `json:"recursive_nameservers" binding:"omitempty,dive,hostname_port"`
	HTTPChallengePort    string   `json:"http_challenge_port"`
}

var CertSettings = &Cert{
	Email:                "",
	CADir:                "",
	RenewalInterval:      7,
	RecursiveNameservers: []string{},
	HTTPChallengePort:    "9180",
}

func (s *Cert) GetCADir() string {
	if s.CADir != "" {
		return s.CADir
	}
	return lego.LEDirectoryProduction
}

func (s *Cert) GetCertRenewalInterval() int {
	if s.RenewalInterval < 7 {
		return 7
	}
	if s.RenewalInterval > 21 {
		return 21
	}
	return s.RenewalInterval
}
