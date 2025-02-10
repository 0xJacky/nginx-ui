package settings

type HTTP struct {
	GithubProxy        string `json:"github_proxy" binding:"omitempty,url"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" protected:"true"`
}

var HTTPSettings = &HTTP{}
