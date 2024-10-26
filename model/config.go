package model

type Config struct {
	Model
	Name          string   `json:"name"`
	Filepath      string   `json:"filepath"`
	SyncNodeIds   []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	SyncOverwrite bool     `json:"sync_overwrite"`
}
