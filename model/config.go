package model

type Config struct {
	Model
	Name     string `json:"name"`
	Filepath string `json:"filepath"`
	// IsDir marks a directory level deployment record. Files stored below such a
	// directory inherit its sync targets, so a whole tree can be replicated at once.
	IsDir         bool     `json:"is_dir"`
	SyncNodeIds   []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	SyncOverwrite bool     `json:"sync_overwrite"`
}
