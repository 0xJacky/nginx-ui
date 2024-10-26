package model

type SiteCategory struct {
	Model
	Name        string   `json:"name"`
	SyncNodeIds []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
}
