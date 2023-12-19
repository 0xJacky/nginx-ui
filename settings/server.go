package settings

type Server struct {
	HttpHost          string `json:"http_host" protected:"true"`
	HttpPort          string `json:"http_port" protected:"true"`
	RunMode           string `json:"run_mode" protected:"true"`
	JwtSecret         string `json:"jwt_secret" protected:"true"`
	NodeSecret        string `json:"node_secret" protected:"true"`
	HTTPChallengePort string `json:"http_challenge_port"`
	Email             string `json:"email" protected:"true"`
	Database          string `json:"database" protected:"true"`
	StartCmd          string `json:"start_cmd" protected:"true"`
	CADir             string `json:"ca_dir"`
	Demo              bool   `json:"demo" protected:"true"`
	PageSize          int    `json:"page_size" protected:"true"`
	GithubProxy       string `json:"github_proxy"`
}

var ServerSettings = Server{
	HttpHost:          "0.0.0.0",
	HttpPort:          "9000",
	RunMode:           "debug",
	HTTPChallengePort: "9180",
	Database:          "database",
	StartCmd:          "login",
	Demo:              false,
	PageSize:          10,
	CADir:             "",
	GithubProxy:       "",
}
