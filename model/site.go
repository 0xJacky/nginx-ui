package model

type Site struct {
	Model
	Path     string `json:"path"`
	Advanced bool   `json:"advanced"`
}
