package settings

import "github.com/go-acme/lego/v5/lego"

type Cert struct {
	Email                string   `json:"email" protected:"true"`
	CADir                string   `json:"ca_dir" binding:"omitempty,url"`
	RenewalInterval      int      `json:"renewal_interval" binding:"min=7,max=21"`
	RecursiveNameservers []string `json:"recursive_nameservers" binding:"omitempty,dive,hostname_port"`
	HTTPChallengePort    string   `json:"http_challenge_port"`
	DiscoveryPatterns    []string `json:"discovery_patterns" binding:"omitempty,dive"`
}

var CertSettings = &Cert{
	Email:                "",
	CADir:                "",
	RenewalInterval:      7,
	RecursiveNameservers: []string{},
	HTTPChallengePort:    "9180",
	DiscoveryPatterns:    []string{},
}

func (s *Cert) GetCADir() string {
	if s.CADir != "" {
		return s.CADir
	}
	return lego.DirectoryURLLetsEncrypt
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
