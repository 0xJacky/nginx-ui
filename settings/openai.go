package settings

type OpenAI struct {
	BaseUrl string `json:"base_url" binding:"omitempty,url"`
	Token   string `json:"token" binding:"omitempty,alphanumdash"`
	Proxy   string `json:"proxy" binding:"omitempty,url"`
	Model   string `json:"model" binding:"omitempty,alphanumdash"`
}

var OpenAISettings = OpenAI{}
