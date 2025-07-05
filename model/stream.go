package model

type Stream struct {
	Model
	Path        string    `json:"path" gorm:"uniqueIndex"`
	Advanced    bool      `json:"advanced"`
	EnvGroupID  uint64    `json:"env_group_id"`
	EnvGroup    *EnvGroup `json:"env_group,omitempty"`
	SyncNodeIDs []uint64  `json:"sync_node_ids" gorm:"serializer:json"`
}

// GetPath implements ConfigEntity interface
func (s *Stream) GetPath() string {
	return s.Path
}

// GetEnvGroupID implements ConfigEntity interface
func (s *Stream) GetEnvGroupID() uint64 {
	return s.EnvGroupID
}

// GetEnvGroup implements ConfigEntity interface
func (s *Stream) GetEnvGroup() *EnvGroup {
	return s.EnvGroup
}
