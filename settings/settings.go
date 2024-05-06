package settings

import (
	"github.com/caarlos0/env/v11"
	"github.com/spf13/cast"
	"gopkg.in/ini.v1"
	"log"
	"os"
	"strings"
	"time"
)

var (
	buildTime    string
	LastModified string

	Conf      *ini.File
	ConfPath  string
	EnvPrefix = "NGINX_UI_"
)

var sections = map[string]interface{}{
	"server":    &ServerSettings,
	"nginx":     &NginxSettings,
	"openai":    &OpenAISettings,
	"casdoor":   &CasdoorSettings,
	"logrotate": &LogrotateSettings,
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
		log.Fatalf("settings.Setup: %v\n", err)
	}
	MapTo()

	parseEnv(&ServerSettings, "SERVER_")
	parseEnv(&NginxSettings, "NGINX_")
	parseEnv(&OpenAISettings, "OPENAI_")
	parseEnv(&CasdoorSettings, "CASDOOR_")
	parseEnv(&LogrotateSettings, "LOGROTATE_")

	// if in official docker, set the restart cmd of nginx to "nginx -s stop",
	// then the supervisor of s6-overlay will start the nginx again.
	if cast.ToBool(os.Getenv("NGINX_UI_OFFICIAL_DOCKER")) {
		NginxSettings.RestartCmd = "nginx -s stop"
	}

	err = Save()
	if err != nil {
		log.Fatalf("settings.Setup: %v\n", err)
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
	return
}

func parseEnv(ptr interface{}, prefix string) {
	err := env.ParseWithOptions(ptr, env.Options{
		Prefix:                EnvPrefix + prefix,
		UseFieldNameByDefault: true,
	})

	if err != nil {
		log.Fatalf("settings.parseEnv: %v\n", err)
	}
}
