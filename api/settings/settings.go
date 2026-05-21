package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/system"
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
	App       cSettings.App      `json:"app"`
	Server    cSettings.Server   `json:"server"`
	Auth      settings.Auth      `json:"auth"`
	Cert      settings.Cert      `json:"cert"`
	Http      settings.HTTP      `json:"http"`
	Node      settings.Node      `json:"node"`
	Openai    settings.OpenAI    `json:"openai"`
	Logrotate settings.Logrotate `json:"logrotate"`
	Nginx     settings.Nginx     `json:"nginx"`
	Oidc      settings.OIDC      `json:"oidc"`
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
		"app":       cSettings.AppSettings,
		"server":    cSettings.ServerSettings,
		"auth":      settings.AuthSettings,
		"casdoor":   settings.CasdoorSettings,
		"cert":      settings.CertSettings,
		"http":      settings.HTTPSettings,
		"logrotate": settings.LogrotateSettings,
		"nginx":     settings.NginxSettings,
		"node":      settings.NodeSettings,
		"openai":    settings.OpenAISettings,
		"terminal":  settings.TerminalSettings,
		"oidc":      settings.OIDCSettings,
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

func buildSettingsResponse() gin.H {
	app := cloneRedactedSettingsSection(cSettings.AppSettings, "jwt_secret")
	openai := cloneRedactedSettingsSection(settings.OpenAISettings)
	openai["provider"] = settings.OpenAISettings.GetProvider()
	if baseURL := settings.OpenAISettings.GetBaseURL(); openai["base_url"] == "" && baseURL != "" {
		openai["base_url"] = baseURL
	}

	return gin.H{
		"app":       app,
		"server":    cSettings.ServerSettings,
		"database":  settings.DatabaseSettings,
		"auth":      cloneRedactedSettingsSection(settings.AuthSettings),
		"casdoor":   cloneRedactedSettingsSection(settings.CasdoorSettings),
		"oidc":      cloneRedactedSettingsSection(settings.OIDCSettings),
		"cert":      cloneRedactedSettingsSection(settings.CertSettings),
		"http":      cloneRedactedSettingsSection(settings.HTTPSettings),
		"logrotate": cloneRedactedSettingsSection(settings.LogrotateSettings),
		"nginx":     buildNginxSettingsResponse(),
		"node":      cloneRedactedSettingsSection(settings.NodeSettings),
		"openai":    openai,
		"terminal":  cloneRedactedSettingsSection(settings.TerminalSettings),
		"webauthn":  settings.WebAuthnSettings,
	}
}

func buildNginxSettingsResponse() gin.H {
	response := cloneRedactedSettingsSection(settings.NginxSettings)
	response["access_log_path"] = nginx.GetAccessLogPath()
	response["error_log_path"] = nginx.GetErrorLogPath()
	response["config_dir"] = nginx.GetConfPath()
	response["pid_path"] = nginx.GetPIDPath()
	response["stub_status_port"] = settings.NginxSettings.GetStubStatusPort()

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
	if payload.App.JwtSecret == redactedSensitiveValue {
		payload.App.JwtSecret = cSettings.AppSettings.JwtSecret
	}

	if payload.Node.Secret == redactedSensitiveValue {
		payload.Node.Secret = settings.NodeSettings.Secret
	}

	if payload.Openai.Token == redactedSensitiveValue {
		payload.Openai.Token = settings.OpenAISettings.Token
	}
}

func GetServerName(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name": settings.NodeSettings.Name,
	})
}

func GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, buildSettingsResponse())
}

func SaveSettings(c *gin.Context) {
	var json saveSettingsPayload

	if !cosy.BindAndValid(c, &json) {
		return
	}

	restoreRedactedSensitiveSettings(&json)

	if settings.LogrotateSettings.Enabled != json.Logrotate.Enabled ||
		settings.LogrotateSettings.Interval != json.Logrotate.Interval {
		go cron.RestartLogrotate()
	}

	// Validate SSL certificates if HTTPS is enabled
	needReloadCert := false
	needRestartProgram := false
	if json.Server.EnableHTTPS != cSettings.ServerSettings.EnableHTTPS {
		needReloadCert = true
		needRestartProgram = true
	}

	if json.Server.SSLCert != cSettings.ServerSettings.SSLCert ||
		json.Server.SSLKey != cSettings.ServerSettings.SSLKey {
		needReloadCert = true
	}

	if json.Server.EnableHTTPS {
		err := system.ValidateSSLCertificates(json.Server.SSLCert, json.Server.SSLKey)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Validate HTTP/2 and HTTP/3 configuration
	if json.Server.EnableH2 && !json.Server.EnableHTTPS {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "HTTP/2 requires HTTPS to be enabled",
		})
		return
	}

	if json.Server.EnableH3 && !json.Server.EnableHTTPS {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "HTTP/3 requires HTTPS to be enabled",
		})
		return
	}

	err := settings.Update(func() {
		cSettings.ProtectedFill(cSettings.AppSettings, &json.App)
		cSettings.ProtectedFill(cSettings.ServerSettings, &json.Server)
		cSettings.ProtectedFill(settings.AuthSettings, &json.Auth)
		cSettings.ProtectedFill(settings.CertSettings, &json.Cert)
		cSettings.ProtectedFill(settings.HTTPSettings, &json.Http)
		cSettings.ProtectedFill(settings.NodeSettings, &json.Node)
		cSettings.ProtectedFill(settings.OpenAISettings, &json.Openai)
		cSettings.ProtectedFill(settings.LogrotateSettings, &json.Logrotate)
		cSettings.ProtectedFill(settings.NginxSettings, &json.Nginx)
		cSettings.ProtectedFill(settings.OIDCSettings, &json.Oidc)
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// If host SSH settings changed, invalidate the cached SSH client so
	// the next nginx command re-dials with the new config.
	nginx.ResetSSHClient()

	GetSettings(c)

	if needReloadCert {
		go func() {
			cert.ReloadServerTLSCertificate()
		}()
	}

	if needRestartProgram {
		go func() {
			risefront.Restart()
		}()
	}
}
