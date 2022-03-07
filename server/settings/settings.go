package settings

import (
	"gopkg.in/ini.v1"
	"log"
)

var Conf *ini.File

var (
	BuildTime string
)

type Server struct {
	HttpPort          string
	RunMode           string
	WebSocketToken    string
	JwtSecret         string
	HTTPChallengePort string
	Email             string
	Database          string
	Demo              bool
}

var ServerSettings = &Server{
	HttpPort:          "9000",
	RunMode:           "debug",
	HTTPChallengePort: "9180",
	Database:          "database",
	Demo:              false,
}

var ConfPath string

var sections = map[string]interface{}{
	"server": ServerSettings,
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
