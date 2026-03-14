package settings

type HTTP struct {
	GithubProxy             string   `json:"github_proxy" binding:"omitempty,url"`
	InsecureSkipVerify      bool     `json:"insecure_skip_verify" protected:"true"`
	WebSocketTrustedOrigins []string `json:"websocket_trusted_origins" binding:"omitempty,dive,url" env:"WEBSOCKET_TRUSTED_ORIGINS"`
}

var HTTPSettings = &HTTP{
	WebSocketTrustedOrigins: []string{},
}
