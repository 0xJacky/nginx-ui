package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/samber/lo"
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

type domainResponse struct {
	ID              uint64             `json:"id"`
	Domain          string             `json:"domain"`
	Description     string             `json:"description"`
	DnsCredentialID uint64             `json:"dns_credential_id"`
	Credential      *credentialSummary `json:"credential,omitempty"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
}

type credentialSummary struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

func newDomainResponse(domain *model.DnsDomain) domainResponse {
	resp := domainResponse{
		ID:              domain.ID,
		Domain:          domain.Domain,
		Description:     domain.Description,
		DnsCredentialID: domain.DnsCredentialID,
		CreatedAt:       domain.CreatedAt.Format(timeFormat),
		UpdatedAt:       domain.UpdatedAt.Format(timeFormat),
	}

	if domain.DnsCredential != nil {
		resp.Credential = lo.ToPtr(credentialSummary{
			ID:       domain.DnsCredential.ID,
			Name:     domain.DnsCredential.Name,
			Provider: domain.DnsCredential.Provider,
		})
	}

	return resp
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
