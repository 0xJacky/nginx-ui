package settings

import (
    "gopkg.in/ini.v1"
    "log"
)

var Conf *ini.File

type Server struct {
    HttpPort       string
    RunMode        string
    WebSocketToken string
    JwtSecret      string
}

var ServerSettings = &Server{}

func Init() {
    var err error
    Conf, err = ini.Load("app.ini")
    if err != nil {
        log.Fatalf("setting.Setup, fail to parse 'app.ini': %v", err)
    }

    mapTo("server", ServerSettings)

}

func mapTo(section string, v interface{}) {
    err := Conf.Section(section).MapTo(v)
    if err != nil {
        log.Fatalf("Cfg.MapTo %s err: %v", section, err)
    }
}
