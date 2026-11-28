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
	"github.com/uozi-tech/cosy/logger"
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
	"DB":             DatabaseSettings,
	"AUTH":           AuthSettings,
	"CASDOOR":        CasdoorSettings,
	"CERT":           CertSettings,
	"CLUSTER":        ClusterSettings,
	"CRYPTO":         CryptoSettings,
	"HTTP":           HTTPSettings,
	"LISTENER":       ListenerSettings,
	"LOGROTATE":      LogrotateSettings,
	"NGINX":          NginxSettings,
	"NGINX_LOG":      NginxLogSettings,
	"NODE":           NodeSettings,
	"OPENAI":         OpenAISettings,
	"SITE_CHECK":     SiteCheckSettings,
	"UPSTREAM_CHECK": UpstreamCheckSettings,
	"INGRESS":        IngressSettings,
	"TERMINAL":       TerminalSettings,
	"WEBAUTHN":       WebAuthnSettings,
	"BACKUP":         BackupSettings,
	"OIDC":           OIDCSettings,
	"PLUGIN":         PluginSettings,
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
	sections.Set("listener", ListenerSettings)
	sections.Set("logrotate", LogrotateSettings)
	sections.Set("nginx", NginxSettings)
	sections.Set("nginx_log", NginxLogSettings)
	sections.Set("node", NodeSettings)
	sections.Set("openai", OpenAISettings)
	sections.Set("site_check", SiteCheckSettings)
	sections.Set("upstream_check", UpstreamCheckSettings)
	sections.Set("ingress", IngressSettings)
	sections.Set("terminal", TerminalSettings)
	sections.Set("webauthn", WebAuthnSettings)
	sections.Set("plugin", PluginSettings)

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

	// In the official image s6-overlay supervises Nginx, so a restart only
	// stops it and the supervisor starts it again. The generic restart would
	// start a daemonized Nginx outside s6 that keeps the ports, and the
	// supervised one then fails to bind forever. NGINX_UI_IGNORE_DOCKER_SOCKET
	// only opts out of the docker socket features and must not change this.
	if helper.IsOfficialDockerImage() && NginxSettings.RestartCmd == "" {
		NginxSettings.RestartCmd = "nginx -s stop"
	}

	if AuthSettings.BanThresholdMinutes <= 0 {
		AuthSettings.BanThresholdMinutes = 10
	}

	if AuthSettings.MaxAttempts <= 0 {
		AuthSettings.MaxAttempts = 10
	}

	if AuthSettings.SecureSessionTimeoutMinutes <= 0 {
		AuthSettings.SecureSessionTimeoutMinutes = DefaultSecureSessionTimeoutMinutes
	}

	reportUnstorableSettings()
}

// Update applies fn to the live settings and saves them to app.ini. When the
// document cannot be stored, nothing is written and the live settings are
// rolled back, so a refused change never stays in effect unsaved: neither a
// value fn set nor a change that a value stored in app.ini itself keeps from
// being written.
func Update(fn func()) (err error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	snapshot := snapshotSettings()
	fn()

	err = saveLocked(snapshot)
	if IsUnstorableValue(err) {
		snapshot.restore()
	}
	return
}

func Save() (err error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	return saveLocked(nil)
}

// saveLocked writes the live settings to app.ini. previous holds the settings
// from before the change being saved, or is nil when none was made.
func saveLocked(previous *settingsSnapshot) (err error) {
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

	if keys := unstorableKeys(settings.Conf); len(keys) > 0 {
		var before *ini.File
		if previous != nil {
			before = previous.document()
		}
		for _, key := range keys {
			if changedSince(before, settings.Conf, key) {
				discardConfChanges()
				return unstorableValueError(key)
			}
		}
		// None of them was set by this change; see keepStoredValues.
		restoreLive := keepStoredValues(settings.Conf, keys)
		defer restoreLive()
	}

	err = saveConfAtomically(settings.Conf, settings.ConfPath)
	if err != nil {
		if IsUnstorableValue(err) {
			// app.ini itself stores a value it cannot write back.
			discardConfChanges()
		}
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

// discardConfChanges reloads the ini document from app.ini, dropping the values
// just reflected into it. If app.ini cannot be read, the current document is
// kept rather than left nil; the next save reflects every setting into it
// again anyway.
func discardConfChanges() {
	current := settings.Conf
	if err := settings.Reload(); err != nil {
		settings.Conf = current
		logger.Error("Reload app.ini: ", err)
	}
}

func saveConfAtomically(conf *ini.File, confPath string) (err error) {
	content, err := encodeConf(conf)
	if err != nil {
		return
	}

	tmpPath := confPath + ".tmp"

	// Same mode ini.File.SaveTo creates the file with.
	err = os.WriteFile(tmpPath, content, 0o666)
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
