package settings

import (
	"github.com/go-acme/lego/v4/lego"
)

type Server struct {
	HttpHost             string   `json:"http_host" protected:"true"`
	HttpPort             string   `json:"http_port" protected:"true"`
	RunMode              string   `json:"run_mode" protected:"true"`
	JwtSecret            string   `json:"jwt_secret" protected:"true"`
	NodeSecret           string   `json:"node_secret" protected:"true"`
	HTTPChallengePort    string   `json:"http_challenge_port"`
	Email                string   `json:"email" protected:"true"`
	Database             string   `json:"database" protected:"true"`
	StartCmd             string   `json:"start_cmd" protected:"true"`
	CADir                string   `json:"ca_dir" binding:"omitempty,url"`
	Demo                 bool     `json:"demo" protected:"true"`
	PageSize             int      `json:"page_size" protected:"true"`
	GithubProxy          string   `json:"github_proxy" binding:"omitempty,url"`
	CertRenewalInterval  int      `json:"cert_renewal_interval" binding:"min=7,max=21"`
	RecursiveNameservers []string `json:"recursive_nameservers" binding:"omitempty,dive,hostname_port"`
	SkipInstallation     bool     `json:"skip_installation" protected:"true"`
	Name                 string   `json:"name" binding:"omitempty,safety_text"`
}

func (s *Server) GetCADir() string {
	if s.Demo {
		return lego.LEDirectoryStaging
	}

	if s.CADir != "" {
		return s.CADir
	}

	return lego.LEDirectoryProduction
}

func (s *Server) GetCertRenewalInterval() int {
	if s.CertRenewalInterval < 7 {
		return 7
	}
	if s.CertRenewalInterval > 21 {
		return 21
	}
	return s.CertRenewalInterval
}

var ServerSettings = Server{
	HttpHost:             "0.0.0.0",
	HttpPort:             "9000",
	RunMode:              "debug",
	HTTPChallengePort:    "9180",
	Database:             "database",
	StartCmd:             "login",
	Demo:                 false,
	PageSize:             10,
	CADir:                "",
	GithubProxy:          "",
	CertRenewalInterval:  7,
	RecursiveNameservers: make([]string, 0),
}
