package settings

type Server struct {
	HttpHost          string `json:"http_host"`
	HttpPort          string `json:"http_port"`
	RunMode           string `json:"run_mode"`
	JwtSecret         string `json:"jwt_secret"`
	NodeSecret        string `json:"node_secret"`
	HTTPChallengePort string `json:"http_challenge_port"`
	Email             string `json:"email"`
	Database          string `json:"database"`
	StartCmd          string `json:"start_cmd"`
	CADir             string `json:"ca_dir"`
	Demo              bool   `json:"demo"`
	PageSize          int    `json:"page_size"`
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
