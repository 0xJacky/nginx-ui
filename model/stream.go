package model

type Stream struct {
	Model
	Path            string                    `json:"path"`
	Advanced        bool                      `json:"advanced"`
	ChatGPTMessages ChatGPTCompletionMessages `json:"chatgpt_messages" gorm:"serializer:json"`
}
