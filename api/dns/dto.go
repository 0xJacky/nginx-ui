package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/model"
)

type domainListQuery struct {
	Keyword      string `form:"keyword"`
	CredentialID uint64 `form:"credential_id"`
	Page         int    `form:"page"`
	PerPage      int    `form:"per_page"`
}

type domainRequest struct {
	Domain          string `json:"domain" binding:"required"`
	Description     string `json:"description"`
	DnsCredentialID uint64 `json:"dns_credential_id" binding:"required"`
}

type recordListQuery struct {
	Type    string `form:"type"`
	Name    string `form:"name"`
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
}

type recordRequest struct {
	Type     string `json:"type" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Content  string `json:"content" binding:"required"`
	TTL      int    `json:"ttl" binding:"required,min=1"`
	Priority *int   `json:"priority"`
	Weight   *int   `json:"weight"`
	Proxied  *bool  `json:"proxied"`
}

func toRecordInput(req recordRequest) dns.RecordInput {
	return dns.RecordInput{
		Type:     req.Type,
		Name:     req.Name,
		Content:  req.Content,
		TTL:      req.TTL,
		Priority: req.Priority,
		Weight:   req.Weight,
		Proxied:  req.Proxied,
	}
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

type ddnsConfigRequest struct {
	Enabled         bool     `json:"enabled"`
	IntervalSeconds int      `json:"interval_seconds" binding:"required,min=60"`
	RecordIDs       []string `json:"record_ids"`
}

type ddnsRecordTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type ddnsConfigResponse struct {
	Enabled         bool               `json:"enabled"`
	IntervalSeconds int                `json:"interval_seconds"`
	Targets         []ddnsRecordTarget `json:"targets"`
	LastIPv4        string             `json:"last_ipv4,omitempty"`
	LastIPv6        string             `json:"last_ipv6,omitempty"`
	LastRunAt       string             `json:"last_run_at,omitempty"`
	LastError       string             `json:"last_error,omitempty"`
}

func toDDNSResponse(cfg *model.DDNSConfig) ddnsConfigResponse {
	resp := ddnsConfigResponse{
		Enabled:         cfg != nil && cfg.Enabled,
		IntervalSeconds: dns.DefaultDDNSInterval(),
		Targets:         []ddnsRecordTarget{},
	}

	if cfg == nil {
		return resp
	}

	interval := cfg.IntervalSeconds
	if interval <= 0 {
		interval = dns.DefaultDDNSInterval()
	}
	resp.IntervalSeconds = interval
	resp.LastIPv4 = cfg.LastIPv4
	resp.LastIPv6 = cfg.LastIPv6
	resp.LastError = cfg.LastError

	if cfg.LastRunAt != nil {
		resp.LastRunAt = cfg.LastRunAt.Format(timeFormat)
	}

	for _, target := range cfg.Targets {
		resp.Targets = append(resp.Targets, ddnsRecordTarget{
			ID:   target.ID,
			Name: target.Name,
			Type: target.Type,
		})
	}

	return resp
}

type ddnsDomainItem struct {
	ID                 uint64             `json:"id"`
	Domain             string             `json:"domain"`
	CredentialName     string             `json:"credential_name,omitempty"`
	CredentialProvider string             `json:"credential_provider,omitempty"`
	Config             ddnsConfigResponse `json:"config"`
}
