package settings

import (
	"gopkg.in/ini.v1"
	"log"
	"os"
)

var Conf *ini.File

type Server struct {
	HttpPort          string
	RunMode           string
	WebSocketToken    string
	JwtSecret         string
	HTTPChallengePort string
	Email             string
	Database          string
}

var ServerSettings = &Server{
	HttpPort:          "9000",
	RunMode:           "debug",
	HTTPChallengePort: "9180",
	Database:          "database",
}

var ConfPath string

func Init(confPath string) {
	ConfPath = confPath
	if _, err := os.Stat(ConfPath); os.IsExist(err) {
		Setup()
	}
}

func Setup() {
	var err error
	Conf, err = ini.Load(ConfPath)
	if err != nil {
		log.Fatalf("setting.Setup, fail to parse '%s': %v", ConfPath, err)
	}

	mapTo("server", ServerSettings)
}

func mapTo(section string, v interface{}) {
	err := Conf.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("Cfg.MapTo %s err: %v", section, err)
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
