package model

type Stream struct {
	Model
	Path     string `json:"path"`
	Advanced bool   `json:"advanced"`
}
