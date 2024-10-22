package model

type SiteCategory struct {
	Model
	Name        string `json:"name"`
	SyncNodeIds []int  `json:"sync_node_ids" gorm:"serializer:json"`
}
