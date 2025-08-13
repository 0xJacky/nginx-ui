package model

type Site struct {
	Model
	Path        string     `json:"path" gorm:"uniqueIndex"`
	Advanced    bool       `json:"advanced"`
	NamespaceID uint64     `json:"namespace_id"`
	Namespace   *Namespace `json:"namespace,omitempty"`
	SyncNodeIDs []uint64   `json:"sync_node_ids" gorm:"serializer:json"`
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
