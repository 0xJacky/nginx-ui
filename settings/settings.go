package settings

import (
    "gopkg.in/ini.v1"
    "log"
    "os"
    "path"
)

var Conf *ini.File

type Server struct {
	HttpPort          string
	RunMode           string
	WebSocketToken    string
	JwtSecret         string
	HTTPChallengePort string
	Email             string
}

var ServerSettings = &Server{}

var DataDir string
var confPath string

func Init(dataDir string)  {
    DataDir = dataDir
    confPath = path.Join(dataDir, "app.ini")
    if _, err := os.Stat(confPath); os.IsNotExist(err) {
        confPath = path.Join(dataDir, "app.example.ini")
    }
    Setup()
}

func Setup()  {
    var err error
    Conf, err = ini.Load(confPath)
    if err != nil {
        log.Fatalf("setting.Setup, fail to parse '%s': %v", confPath, err)
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
    confPath = path.Join(DataDir, "app.ini")
    err = Conf.SaveTo(confPath)
    if err != nil {
        return
    }
    Setup()
    return
}
