package settings

import (
	"github.com/spf13/cast"
	"gopkg.in/ini.v1"
	"log"
	"strings"
	"time"
)

var Conf *ini.File

var (
	buildTime    string
	LastModified string
)

type Server struct {
	HttpHost            string `json:"http_host"`
	HttpPort            string `json:"http_port"`
	RunMode             string `json:"run_mode"`
	JwtSecret           string `json:"jwt_secret"`
	NodeSecret          string `json:"node_secret"`
	HTTPChallengePort   string `json:"http_challenge_port"`
	Email               string `json:"email"`
	Database            string `json:"database"`
	StartCmd            string `json:"start_cmd"`
	CADir               string `json:"ca_dir"`
	Demo                bool   `json:"demo"`
	PageSize            int    `json:"page_size"`
	GithubProxy         string `json:"github_proxy"`
	CasdoorEndpoint     string `json:"casdoor_endpoint"`
	CasdoorClientId     string `json:"casdoor_client_id"`
	CasdoorClientSecret string `json:"casdoor_client_secret"`
	CasdoorCertificate  string `json:"casdoor_certificate"`
	CasdoorOrganization string `json:"casdoor_organization"`
	CasdoorApplication  string `json:"casdoor_application"`
	CasdoorRedirectUri  string `json:"casdoor_redirect_uri"`
}

type Nginx struct {
	AccessLogPath string `json:"access_log_path"`
	ErrorLogPath  string `json:"error_log_path"`
	ConfigDir     string `json:"config_dir"`
	PIDPath       string `json:"pid_path"`
	TestConfigCmd string `json:"test_config_cmd"`
	ReloadCmd     string `json:"reload_cmd"`
	RestartCmd    string `json:"restart_cmd"`
}

type OpenAI struct {
	BaseUrl string `json:"base_url"`
	Token   string `json:"token"`
	Proxy   string `json:"proxy"`
	Model   string `json:"model"`
}

var ServerSettings = Server{
	HttpHost:            "0.0.0.0",
	HttpPort:            "9000",
	RunMode:             "debug",
	HTTPChallengePort:   "9180",
	Database:            "database",
	StartCmd:            "login",
	Demo:                false,
	PageSize:            10,
	CADir:               "",
	GithubProxy:         "",
	CasdoorEndpoint:     "",
	CasdoorClientId:     "",
	CasdoorClientSecret: "",
	CasdoorCertificate:  "",
	CasdoorOrganization: "",
	CasdoorApplication:  "",
	CasdoorRedirectUri:  "",
}

var NginxSettings = Nginx{
	AccessLogPath: "",
	ErrorLogPath:  "",
}

var OpenAISettings = OpenAI{}

var ConfPath string

var sections = map[string]interface{}{
	"server": &ServerSettings,
	"nginx":  &NginxSettings,
	"openai": &OpenAISettings,
}

func init() {
	t := time.Unix(cast.ToInt64(buildTime), 0)
	LastModified = strings.ReplaceAll(t.Format(time.RFC1123), "UTC", "GMT")
}

func Init(confPath string) {
	ConfPath = confPath
	Setup()
}

func Setup() {
	var err error
	Conf, err = ini.LooseLoad(ConfPath)
	if err != nil {
		log.Fatalf("setting.Setup: %v\n", err)
	}
	MapTo()
}

func MapTo() {
	for k, v := range sections {
		mapTo(k, v)
	}
}

func ReflectFrom() {
	for k, v := range sections {
		reflectFrom(k, v)
	}
}

func mapTo(section string, v interface{}) {
	err := Conf.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("Cfg.MapTo %s err: %v", section, err)
	}
}

func reflectFrom(section string, v interface{}) {
	log.Print(section, v)
	err := Conf.Section(section).ReflectFrom(v)
	if err != nil {
		log.Fatalf("Cfg.ReflectFrom %s err: %v", section, err)
	}
}

func Save() (err error) {
	err = Conf.SaveTo(ConfPath)
	if err != nil {
		return
	}
	Setup()
	return
}
