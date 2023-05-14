package model

type Environment struct {
	Model
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token"`
}
