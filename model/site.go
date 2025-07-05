package model

type Site struct {
	Model
	Path        string    `json:"path" gorm:"uniqueIndex"`
	Advanced    bool      `json:"advanced"`
	EnvGroupID  uint64    `json:"env_group_id"`
	EnvGroup    *EnvGroup `json:"env_group,omitempty"`
	SyncNodeIDs []uint64  `json:"sync_node_ids" gorm:"serializer:json"`
}

// GetPath implements ConfigEntity interface
func (s *Site) GetPath() string {
	return s.Path
}

// GetEnvGroupID implements ConfigEntity interface
func (s *Site) GetEnvGroupID() uint64 {
	return s.EnvGroupID
}

// GetEnvGroup implements ConfigEntity interface
func (s *Site) GetEnvGroup() *EnvGroup {
	return s.EnvGroup
}
