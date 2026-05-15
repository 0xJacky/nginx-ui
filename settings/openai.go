package settings

import (
	"strings"

	"github.com/sashabaranov/go-openai"
)

const (
	OpenAIProviderOpenAI     = "openai"
	OpenAIProviderAtlasCloud = "atlas_cloud"
	OpenAIProviderCustom     = "custom"
	AtlasCloudBaseURL        = "https://api.atlascloud.ai/v1"
)

type OpenAI struct {
	Provider             string `json:"provider" binding:"omitempty,oneof=openai atlas_cloud custom"`
	BaseUrl              string `json:"base_url" binding:"omitempty,url"`
	Token                string `json:"token" binding:"omitempty,safety_text"`
	Proxy                string `json:"proxy" binding:"omitempty,url"`
	Model                string `json:"model" binding:"omitempty,safety_text"`
	APIType              string `json:"api_type" binding:"omitempty,oneof=OPEN_AI AZURE"`
	EnableCodeCompletion bool   `json:"enable_code_completion" binding:"omitempty"`
	CodeCompletionModel  string `json:"code_completion_model" binding:"omitempty,safety_text"`
}

var OpenAISettings = &OpenAI{
	Provider: OpenAIProviderOpenAI,
	APIType:  string(openai.APITypeOpenAI),
}

func (o *OpenAI) GetCodeCompletionModel() string {
	if o.CodeCompletionModel == "" {
		return o.Model
	}
	return o.CodeCompletionModel
}

func (o *OpenAI) GetProvider() string {
	if o == nil {
		return OpenAIProviderOpenAI
	}

	switch normalizeOpenAIBaseURL(o.BaseUrl) {
	case AtlasCloudBaseURL:
		return OpenAIProviderAtlasCloud
	}

	if o.Provider == "" {
		return OpenAIProviderOpenAI
	}

	return o.Provider
}

func (o *OpenAI) GetBaseURL() string {
	if o == nil {
		return ""
	}

	if baseURL := normalizeOpenAIBaseURL(o.BaseUrl); baseURL != "" {
		return baseURL
	}

	switch o.GetProvider() {
	case OpenAIProviderAtlasCloud:
		return AtlasCloudBaseURL
	default:
		return ""
	}
}

func normalizeOpenAIBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}
