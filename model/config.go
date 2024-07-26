package model

type Config struct {
	Model
	Name        string `json:"name"`
	Filepath    string `json:"filepath"`
	SyncNodeIds []int  `json:"sync_node_ids" gorm:"serializer:json"`
    SyncOverwrite  bool `json:"sync_overwrite"`
}
