package model

type Log struct {
	Model
	Title   string `json:"title"`
	Content string `json:"content"`
}
