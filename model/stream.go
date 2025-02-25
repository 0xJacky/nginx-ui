package model

type Stream struct {
	Model
	Path        string   `json:"path"`
	Advanced    bool     `json:"advanced"`
	SyncNodeIDs []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
}
