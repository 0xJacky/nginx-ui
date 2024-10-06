package settings

import (
	"github.com/caarlos0/env/v11"
	"github.com/spf13/cast"
	"gopkg.in/ini.v1"
	"log"
	"os"
	"reflect"
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
	"cluster":   &ClusterSettings,
	"auth":      &AuthSettings,
	"crypto":    &CryptoSettings,
	"webauthn":  &WebAuthnSettings,
}

func init() {
	t := time.Unix(cast.ToInt64(buildTime), 0)
	LastModified = strings.ReplaceAll(t.Format(time.RFC1123), "UTC", "GMT")
}

func Init(confPath string) {
	ConfPath = confPath
	Setup()
}

func load() (err error) {
	Conf, err = ini.LoadSources(ini.LoadOptions{
		Loose:        true,
		AllowShadows: true,
	}, ConfPath)

	return
}

func Setup() {
	err := load()

	if err != nil {
		log.Fatalf("settings.Setup: %v\n", err)
	}

	MapTo()

	parseEnv(&ServerSettings, "SERVER_")
	parseEnv(&NginxSettings, "NGINX_")
	parseEnv(&OpenAISettings, "OPENAI_")
	parseEnv(&CasdoorSettings, "CASDOOR_")
	parseEnv(&LogrotateSettings, "LOGROTATE_")
	parseEnv(&AuthSettings, "AUTH_")
	parseEnv(&CryptoSettings, "CRYPTO_")
	parseEnv(&WebAuthnSettings, "WEBAUTHN_")

	// if in official docker, set the restart cmd of nginx to "nginx -s stop",
	// then the supervisor of s6-overlay will start the nginx again.
	if cast.ToBool(os.Getenv("NGINX_UI_OFFICIAL_DOCKER")) {
		NginxSettings.RestartCmd = "nginx -s stop"
	}

	if AuthSettings.BanThresholdMinutes <= 0 {
		AuthSettings.BanThresholdMinutes = 10
	}

	if AuthSettings.MaxAttempts <= 0 {
		AuthSettings.MaxAttempts = 10
	}
}

func MapTo() {
	for k, v := range sections {
		err := mapTo(k, v)

		if err != nil {
			log.Fatalf("Cfg.MapTo %s err: %v", k, err)
		}
	}
}

func Save() (err error) {
	for k, v := range sections {
		reflectFrom(k, v)
	}

	// fix unable to save empty slice
	if len(ServerSettings.RecursiveNameservers) == 0 {
		Conf.Section("server").Key("RecursiveNameservers").SetValue("")
	}

	err = Conf.SaveTo(ConfPath)
	if err != nil {
		return
	}
	return
}

func ProtectedFill(targetSettings interface{}, newSettings interface{}) {
	s := reflect.TypeOf(targetSettings).Elem()
	vt := reflect.ValueOf(targetSettings).Elem()
	vn := reflect.ValueOf(newSettings).Elem()

	// copy the values from new to target settings if it is not protected
	for i := 0; i < s.NumField(); i++ {
		if s.Field(i).Tag.Get("protected") != "true" {
			vt.Field(i).Set(vn.Field(i))
		}
	}
}

func mapTo(section string, v interface{}) error {
	return Conf.Section(section).MapTo(v)
}

func reflectFrom(section string, v interface{}) {
	err := Conf.Section(section).ReflectFrom(v)
	if err != nil {
		log.Fatalf("Cfg.ReflectFrom %s err: %v", section, err)
	}
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
