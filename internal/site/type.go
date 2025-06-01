package site

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
)

type SiteStatus string

const (
	SiteStatusEnabled     SiteStatus = "enabled"
	SiteStatusDisabled    SiteStatus = "disabled"
	SiteStatusMaintenance SiteStatus = "maintenance"
)

// ProxyTarget is an alias for upstream.ProxyTarget
type ProxyTarget = upstream.ProxyTarget

type Site struct {
	*model.Site
	Name         string               `json:"name"`
	ModifiedAt   time.Time            `json:"modified_at"`
	Status       SiteStatus           `json:"status"`
	Config       string               `json:"config"`
	AutoCert     bool                 `json:"auto_cert"`
	Tokenized    *nginx.NgxConfig     `json:"tokenized,omitempty"`
	CertInfo     map[int][]*cert.Info `json:"cert_info,omitempty"`
	Filepath     string               `json:"filepath"`
	ProxyTargets []ProxyTarget        `json:"proxy_targets,omitempty"`
}
