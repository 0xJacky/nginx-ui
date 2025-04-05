package model

type Stream struct {
	Model
	Path        string    `json:"path" gorm:"uniqueIndex"`
	Advanced    bool      `json:"advanced"`
	EnvGroupID  uint64    `json:"env_group_id"`
	EnvGroup    *EnvGroup `json:"env_group,omitempty"`
	SyncNodeIDs []uint64  `json:"sync_node_ids" gorm:"serializer:json"`
}
