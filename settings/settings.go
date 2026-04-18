package settings

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/caarlos0/env/v11"
	"github.com/elliotchance/orderedmap/v3"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

var (
	buildTime    string
	LastModified string
	EnvPrefix    = "NGINX_UI_"
	settingsMu   sync.Mutex
)

var sections = orderedmap.NewOrderedMap[string, any]()

var envPrefixMap = map[string]interface{}{
	// Cosy
	"APP":    settings.AppSettings,
	"SERVER": settings.ServerSettings,
	// Nginx UI
	"DB":         DatabaseSettings,
	"AUTH":       AuthSettings,
	"CASDOOR":    CasdoorSettings,
	"CERT":       CertSettings,
	"CLUSTER":    ClusterSettings,
	"CRYPTO":     CryptoSettings,
	"HTTP":       HTTPSettings,
	"LOGROTATE":  LogrotateSettings,
	"NGINX":      NginxSettings,
	"NGINX_LOG":  NginxLogSettings,
	"NODE":       NodeSettings,
	"OPENAI":     OpenAISettings,
	"SITE_CHECK": SiteCheckSettings,
	"TERMINAL":   TerminalSettings,
	"WEBAUTHN":   WebAuthnSettings,
	"BACKUP":     BackupSettings,
	"OIDC":       OIDCSettings,
}

func init() {
	t := time.Unix(cast.ToInt64(buildTime), 0)
	LastModified = strings.ReplaceAll(t.Format(time.RFC1123), "UTC", "GMT")

	sections.Set("database", DatabaseSettings)
	sections.Set("auth", AuthSettings)
	sections.Set("backup", BackupSettings)
	sections.Set("casdoor", CasdoorSettings)
	sections.Set("oidc", OIDCSettings)
	sections.Set("cert", CertSettings)
	sections.Set("cluster", ClusterSettings)
	sections.Set("crypto", CryptoSettings)
	sections.Set("http", HTTPSettings)
	sections.Set("logrotate", LogrotateSettings)
	sections.Set("nginx", NginxSettings)
	sections.Set("nginx_log", NginxLogSettings)
	sections.Set("node", NodeSettings)
	sections.Set("openai", OpenAISettings)
	sections.Set("site_check", SiteCheckSettings)
	sections.Set("terminal", TerminalSettings)
	sections.Set("webauthn", WebAuthnSettings)

	for k, v := range sections.AllFromFront() {
		settings.Register(k, v)
	}
	settings.WithoutRedis()
	settings.WithoutSonyflake()
}

func Init(confPath string) {
	migrate(confPath)

	settings.Init(confPath)

	// Set Default Port
	if settings.ServerSettings.Port == 0 {
		settings.ServerSettings.Port = 9000
	}

	for prefix, ptr := range envPrefixMap {
		parseEnv(ptr, prefix+"_")
	}

	// if in official docker, set the restart cmd of nginx to "nginx -s stop",
	// then the supervisor of s6-overlay will start the nginx again.
	if helper.InNginxUIOfficialDocker() {
		NginxSettings.RestartCmd = "nginx -s stop"
	}

	if AuthSettings.BanThresholdMinutes <= 0 {
		AuthSettings.BanThresholdMinutes = 10
	}

	if AuthSettings.MaxAttempts <= 0 {
		AuthSettings.MaxAttempts = 10
	}
}

func Update(fn func()) (err error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	fn()

	return saveLocked()
}

func Save() (err error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	return saveLocked()
}

func saveLocked() (err error) {
	// "fix" unable to save empty slice
	if len(CertSettings.RecursiveNameservers) == 0 {
		settings.Conf.Section("cert").Key("RecursiveNameservers").SetValue("")
	}

	settings.ReflectFrom("app", settings.AppSettings)
	settings.ReflectFrom("server", settings.ServerSettings)
	settings.ReflectFrom("log", settings.LogSettings)
	settings.ReflectFrom("sls", settings.SLSSettings)

	for name, ptr := range sections.AllFromFront() {
		settings.ReflectFrom(name, ptr)
	}

	err = saveConfAtomically(settings.Conf, settings.ConfPath)
	if err != nil {
		return
	}

	err = settings.Reload()
	if err != nil {
		return
	}

	err = settings.MapTo("app", settings.AppSettings)
	if err != nil {
		return
	}

	err = settings.MapTo("server", settings.ServerSettings)
	if err != nil {
		return
	}

	err = settings.MapTo("log", settings.LogSettings)
	if err != nil {
		return
	}

	err = settings.MapTo("sls", settings.SLSSettings)
	if err != nil {
		return
	}

	for name, ptr := range sections.AllFromFront() {
		err = settings.MapTo(name, ptr)
		if err != nil {
			return
		}
	}

	return
}

func saveConfAtomically(conf *ini.File, confPath string) (err error) {
	tmpPath := confPath + ".tmp"

	err = conf.SaveTo(tmpPath)
	if err != nil {
		return
	}

	err = os.Rename(tmpPath, confPath)
	if err != nil {
		_ = os.Remove(tmpPath)
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
