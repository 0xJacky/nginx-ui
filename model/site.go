package model

type SiteDNSRecord struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Exists bool   `json:"exists"`
}

type Site struct {
	Model
	Path        string          `json:"path" gorm:"uniqueIndex"`
	Advanced    bool            `json:"advanced"`
	NamespaceID uint64          `json:"namespace_id"`
	Namespace   *Namespace      `json:"namespace,omitempty"`
	SyncNodeIDs []uint64        `json:"sync_node_ids" gorm:"serializer:json"`
	DNSRecords  []SiteDNSRecord `json:"dns_records" gorm:"serializer:json"`
	// Legacy single-record fields are kept for backward compatibility.
	DNSDomainID     *int    `json:"dns_domain_id"`     // Linked DNS domain ID
	DNSRecordID     *string `json:"dns_record_id"`     // Linked DNS record ID
	DNSRecordName   *string `json:"dns_record_name"`   // Cached DNS record name
	DNSRecordType   *string `json:"dns_record_type"`   // Cached DNS record type (A, AAAA, CNAME)
	DNSRecordExists *bool   `json:"dns_record_exists"` // Whether the DNS record still exists
}

// GetPath implements ConfigEntity interface
func (s *Site) GetPath() string {
	return s.Path
}

// GetNamespaceID implements ConfigEntity interface
func (s *Site) GetNamespaceID() uint64 {
	return s.NamespaceID
}

// GetNamespace implements ConfigEntity interface
func (s *Site) GetNamespace() *Namespace {
	return s.Namespace
}
