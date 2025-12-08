package model

// DnsDomain represents a managed domain bound to a DNS credential.
type DnsDomain struct {
	Model

	Domain          string         `json:"domain" gorm:"size:255;not null;uniqueIndex:idx_dns_domain_credential"`
	Description     string         `json:"description"`
	DnsCredentialID uint64         `json:"dns_credential_id" gorm:"not null;uniqueIndex:idx_dns_domain_credential"`
	DnsCredential   *DnsCredential `json:"dns_credential,omitempty" gorm:"constraint:OnDelete:CASCADE;"`
	DDNSConfig      *DDNSConfig    `json:"ddns_config,omitempty" gorm:"column:ddns_config;serializer:json"`
}
