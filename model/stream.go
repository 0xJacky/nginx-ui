package model

type Stream struct {
	Model
	Path        string     `json:"path" gorm:"uniqueIndex"`
	Advanced    bool       `json:"advanced"`
	NamespaceID uint64     `json:"namespace_id"`
	Namespace   *Namespace `json:"namespace,omitempty"`
	SyncNodeIDs []uint64   `json:"sync_node_ids" gorm:"serializer:json"`
}

// GetPath implements ConfigEntity interface
func (s *Stream) GetPath() string {
	return s.Path
}

// GetNamespaceID implements ConfigEntity interface
func (s *Stream) GetNamespaceID() uint64 {
	return s.NamespaceID
}

// GetNamespace implements ConfigEntity interface
func (s *Stream) GetNamespace() *Namespace {
	return s.Namespace
}
