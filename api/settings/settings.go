package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const redactedSensitiveValue = settings.RedactedSensitiveValue

var manuallySensitiveSettingGetters = map[string]func() any{
	"app.jwt_secret": func() any {
		return cSettings.AppSettings.JwtSecret
	},
}

type saveSettingsPayload struct {
	App           cSettings.App          `json:"app"`
	Server        cSettings.Server       `json:"server"`
	Auth          settings.Auth          `json:"auth"`
	Cert          settings.Cert          `json:"cert"`
	Http          settings.HTTP          `json:"http"`
	Node          settings.Node          `json:"node"`
	Openai        settings.OpenAI        `json:"openai"`
	Logrotate     settings.Logrotate     `json:"logrotate"`
	Nginx         settings.Nginx         `json:"nginx"`
	Oidc          settings.OIDC          `json:"oidc"`
	SiteCheck     settings.SiteCheck     `json:"site_check"`
	UpstreamCheck settings.UpstreamCheck `json:"upstream_check"`
}

func cloneSettingsSection(section any) gin.H {
	raw, err := json.Marshal(section)
	if err != nil {
		return gin.H{}
	}

	var cloned gin.H
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return gin.H{}
	}

	return cloned
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "-" {
		return ""
	}
	if name != "" {
		return name
	}
	return field.Name
}

func shouldRedactSensitiveValue(value reflect.Value) bool {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.String, reflect.Slice, reflect.Array:
		return true
	default:
		return false
	}
}

func redactSensitiveValue(value reflect.Value) any {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return redactedSensitiveValue
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return []string{redactedSensitiveValue}
	default:
		return redactedSensitiveValue
	}
}

func redactSensitiveFields(section any, cloned gin.H) {
	value := reflect.ValueOf(section)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return
	}

	valueType := value.Type()
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.Tag.Get("sensitive") != "true" {
			continue
		}

		fieldValue := value.Field(i)
		if !shouldRedactSensitiveValue(fieldValue) {
			continue
		}

		name := jsonFieldName(field)
		if name == "" {
			continue
		}
		cloned[name] = redactSensitiveValue(fieldValue)
	}
}

func cloneRedactedSettingsSection(section any, extraSensitiveFields ...string) gin.H {
	cloned := cloneSettingsSection(section)
	redactSensitiveFields(section, cloned)
	for _, field := range extraSensitiveFields {
		cloned[field] = redactedSensitiveValue
	}
	return cloned
}

func settingsSectionSources() map[string]any {
	return map[string]any{
		"app":            cSettings.AppSettings,
		"server":         cSettings.ServerSettings,
		"auth":           settings.AuthSettings,
		"casdoor":        settings.CasdoorSettings,
		"cert":           settings.CertSettings,
		"http":           settings.HTTPSettings,
		"logrotate":      settings.LogrotateSettings,
		"nginx":          settings.NginxSettings,
		"nginx_log":      settings.NginxLogSettings,
		"node":           settings.NodeSettings,
		"openai":         settings.OpenAISettings,
		"terminal":       settings.TerminalSettings,
		"oidc":           settings.OIDCSettings,
		"site_check":     settings.SiteCheckSettings,
		"upstream_check": settings.UpstreamCheckSettings,
	}
}

func getProtectedSettingValue(path string) (any, bool) {
	if getter, ok := manuallySensitiveSettingGetters[path]; ok {
		return getter(), true
	}

	sectionName, fieldName, ok := strings.Cut(path, ".")
	if !ok || sectionName == "" || fieldName == "" {
		return nil, false
	}

	section, ok := settingsSectionSources()[sectionName]
	if !ok {
		return nil, false
	}

	value := reflect.ValueOf(section)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil, false
	}

	valueType := value.Type()
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.Tag.Get("sensitive") != "true" || jsonFieldName(field) != fieldName {
			continue
		}
		return value.Field(i).Interface(), true
	}

	return nil, false
}

// settingsResponseBuilders renders each section of the GET /settings response.
// A section save answers with its own entry only, so a request never reads
// sections that a concurrent save of another group may be writing.
var settingsResponseBuilders = map[string]func() any{
	"app": func() any {
		return cloneRedactedSettingsSection(cSettings.AppSettings, "jwt_secret")
	},
	"server":   func() any { return cSettings.ServerSettings },
	"listener": func() any { return settings.ListenerSettings },
	"database": func() any { return settings.DatabaseSettings },
	"auth":     func() any { return cloneRedactedSettingsSection(settings.AuthSettings) },
	"casdoor":  func() any { return cloneRedactedSettingsSection(settings.CasdoorSettings) },
	"oidc":     func() any { return cloneRedactedSettingsSection(settings.OIDCSettings) },
	"cert":     func() any { return cloneRedactedSettingsSection(settings.CertSettings) },
	"http":     func() any { return cloneRedactedSettingsSection(settings.HTTPSettings) },
	"logrotate": func() any {
		return cloneRedactedSettingsSection(settings.LogrotateSettings)
	},
	"nginx":     func() any { return buildNginxSettingsResponse() },
	"nginx_log": func() any { return cloneRedactedSettingsSection(settings.NginxLogSettings) },
	"node":      func() any { return cloneRedactedSettingsSection(settings.NodeSettings) },
	"openai":    func() any { return buildOpenAISettingsResponse() },
	"terminal":  func() any { return cloneRedactedSettingsSection(settings.TerminalSettings) },
	"webauthn":  func() any { return settings.WebAuthnSettings },
	"site_check": func() any {
		return settings.SiteCheckSettings
	},
	"upstream_check": func() any {
		return settings.UpstreamCheckSettings
	},
}

func buildSettingsResponse() gin.H {
	response := make(gin.H, len(settingsResponseBuilders))
	for name, build := range settingsResponseBuilders {
		response[name] = build()
	}
	return response
}

func buildSettingsSectionResponse(name string) any {
	build, ok := settingsResponseBuilders[name]
	if !ok {
		return gin.H{}
	}
	return build()
}

func buildOpenAISettingsResponse() gin.H {
	openai := cloneRedactedSettingsSection(settings.OpenAISettings)
	openai["provider"] = settings.OpenAISettings.GetProvider()
	if baseURL := settings.OpenAISettings.GetBaseURL(); openai["base_url"] == "" && baseURL != "" {
		openai["base_url"] = baseURL
	}
	return openai
}

func buildNginxSettingsResponse() gin.H {
	response := cloneRedactedSettingsSection(settings.NginxSettings)
	response["access_log_path"] = nginx.GetAccessLogPath()
	response["error_log_path"] = nginx.GetErrorLogPath()
	response["config_dir"] = nginx.GetConfPath()
	response["pid_path"] = nginx.GetPIDPath()
	response["stub_status_port"] = settings.NginxSettings.GetStubStatusPort()
	response["maintenance_dir"] = settings.NginxSettings.GetMaintenanceDir()

	if settings.NginxSettings.ReloadCmd == "" {
		response["reload_cmd"] = "nginx -s reload"
	}

	if settings.NginxSettings.RestartCmd == "" {
		pidPath := nginx.GetPIDPath()
		if daemon := nginx.GetSbinPath(); daemon != "" {
			response["restart_cmd"] =
				fmt.Sprintf("start-stop-daemon --start --quiet --pidfile %s --exec %s", pidPath, daemon)
		} else {
			response["restart_cmd"] =
				fmt.Sprintf("start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5"+
					" --pidfile %s && nginx", pidPath)
		}
	}

	return response
}

func restoreRedactedSensitiveSettings(payload *saveSettingsPayload) {
	appSection.restore(&payload.App)
	nodeSection.restore(&payload.Node)
	openAISection.restore(&payload.Openai)
}

func GetServerName(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name": settings.NodeSettings.Name,
	})
}

func GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, buildSettingsResponse())
}

// SaveSettings saves every writable section at once. The preference page saves
// each section through SaveSettingsSection instead; this combined endpoint
// stays for API clients and scripts that post the whole document.
func SaveSettings(c *gin.Context) {
	var json saveSettingsPayload
	json.Auth = *settings.AuthSettings

	if !cosy.BindAndValid(c, &json) {
		return
	}
	if !authorizeMFAPolicy(c, &json.Auth) {
		return
	}

	saves := []*sectionSave{
		appSection.bind(&json.App),
		serverSection.bind(&json.Server),
		authSection.bind(&json.Auth),
		certSection.bind(&json.Cert),
		httpSection.bind(&json.Http),
		nodeSection.bind(&json.Node),
		openAISection.bind(&json.Openai),
		logrotateSection.bind(&json.Logrotate),
		nginxSection.bind(&json.Nginx),
		oidcSection.bind(&json.Oidc),
		siteCheckSection.bind(&json.SiteCheck),
		upstreamCheckSection.bind(&json.UpstreamCheck),
	}

	if !persistSections(c, saves...) {
		return
	}

	GetSettings(c)

	for _, save := range saves {
		save.afterSave()
	}
}
