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

var ConfPath string

var sections = map[string]interface{}{
	"server":  &ServerSettings,
	"nginx":   &NginxSettings,
	"openai":  &OpenAISettings,
	"casdoor": &CasdoorSettings,
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
