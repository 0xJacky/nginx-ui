package model

// EnvGroup represents a group of environments that can be synced across nodes
type EnvGroup struct {
	Model
	Name        string   `json:"name"`
	SyncNodeIds []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	OrderID     int      `json:"-" gorm:"default:0"`
}
