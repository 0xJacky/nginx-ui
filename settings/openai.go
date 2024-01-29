package settings

type OpenAI struct {
	BaseUrl string `json:"base_url" binding:"omitempty,url"`
	Token   string `json:"token" binding:"omitempty,alpha_num_dash_dot"`
	Proxy   string `json:"proxy" binding:"omitempty,url"`
	Model   string `json:"model" binding:"omitempty,alpha_num_dash_dot"`
}

var OpenAISettings = OpenAI{}
