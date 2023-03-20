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
	HttpPort          string `json:"http_port"`
	RunMode           string `json:"run_mode"`
	JwtSecret         string `json:"jwt_secret"`
	HTTPChallengePort string `json:"http_challenge_port"`
	Email             string `json:"email"`
	Database          string `json:"database"`
	StartCmd          string `json:"start_cmd"`
	CADir             string `json:"ca_dir"`
	Demo              bool   `json:"demo"`
	PageSize          int    `json:"page_size"`
	GithubProxy       string `json:"github_proxy"`
	NginxConfigDir    string `json:"nginx_config_dir"`
	NginxPIDPath      string `json:"nginx_pid_path"`
}

type NginxLog struct {
	AccessLogPath string `json:"access_log_path"`
	ErrorLogPath  string `json:"error_log_path"`
}

type OpenAI struct {
	BaseUrl string `json:"base_url"`
	Token   string `json:"token"`
	Proxy   string `json:"proxy"`
	Model   string `json:"model"`
}

var ServerSettings = &Server{
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

var NginxLogSettings = &NginxLog{
	AccessLogPath: "",
	ErrorLogPath:  "",
}

var OpenAISettings = &OpenAI{}

var ConfPath string

var sections = map[string]interface{}{
	"server":    ServerSettings,
	"nginx_log": NginxLogSettings,
	"openai":    OpenAISettings,
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
		log.Printf("setting.Setup: %v", err)
	} else {
		MapTo()
	}
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
