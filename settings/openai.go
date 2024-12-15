package settings

import "github.com/sashabaranov/go-openai"

type OpenAI struct {
	BaseUrl string `json:"base_url" binding:"omitempty,url"`
	Token   string `json:"token" binding:"omitempty,safety_text"`
	Proxy   string `json:"proxy" binding:"omitempty,url"`
	Model   string `json:"model" binding:"omitempty,safety_text"`
	APIType string `json:"api_type" binding:"omitempty,oneof=OPEN_AI AZURE"`
}

var OpenAISettings = &OpenAI{
	APIType: string(openai.APITypeOpenAI),
}
