package settings

type OpenAI struct {
	BaseUrl string `json:"base_url"`
	Token   string `json:"token"`
	Proxy   string `json:"proxy"`
	Model   string `json:"model"`
}

var OpenAISettings = OpenAI{}
