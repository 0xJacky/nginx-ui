package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/dns"
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
