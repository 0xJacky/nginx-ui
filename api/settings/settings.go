package settings

import (
	"fmt"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func GetServerName(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name": settings.NodeSettings.Name,
	})
}

func GetSettings(c *gin.Context) {
	settings.NginxSettings.AccessLogPath = nginx.GetAccessLogPath()
	settings.NginxSettings.ErrorLogPath = nginx.GetErrorLogPath()
	settings.NginxSettings.ConfigDir = nginx.GetConfPath()
	settings.NginxSettings.PIDPath = nginx.GetPIDPath()

	if settings.NginxSettings.ReloadCmd == "" {
		settings.NginxSettings.ReloadCmd = "nginx -s reload"
	}

	if settings.NginxSettings.RestartCmd == "" {
		pidPath := nginx.GetPIDPath()
		daemon := nginx.GetSbinPath()
		if daemon == "" {
			settings.NginxSettings.RestartCmd =
				fmt.Sprintf("start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5"+
					" --pidfile %s && nginx", pidPath)
			return
		}

		settings.NginxSettings.RestartCmd =
			fmt.Sprintf("start-stop-daemon --start --quiet --pidfile %s --exec %s", pidPath, daemon)
	}

	c.JSON(http.StatusOK, gin.H{
		"app":       cSettings.AppSettings,
		"server":    cSettings.ServerSettings,
		"database":  settings.DatabaseSettings,
		"auth":      settings.AuthSettings,
		"casdoor":   settings.CasdoorSettings,
		"cert":      settings.CertSettings,
		"http":      settings.HTTPSettings,
		"logrotate": settings.LogrotateSettings,
		"nginx":     settings.NginxSettings,
		"node":      settings.NodeSettings,
		"openai":    settings.OpenAISettings,
		"terminal":  settings.TerminalSettings,
		"webauthn":  settings.WebAuthnSettings,
	})
}

func SaveSettings(c *gin.Context) {
	var json struct {
		App       cSettings.App      `json:"app"`
		Server    cSettings.Server   `json:"server"`
		Auth      settings.Auth      `json:"auth"`
		Cert      settings.Cert      `json:"cert"`
		Http      settings.HTTP      `json:"http"`
		Node      settings.Node      `json:"node"`
		Openai    settings.OpenAI    `json:"openai"`
		Logrotate settings.Logrotate `json:"logrotate"`
		Nginx     settings.Nginx     `json:"nginx"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	if settings.LogrotateSettings.Enabled != json.Logrotate.Enabled ||
		settings.LogrotateSettings.Interval != json.Logrotate.Interval {
		go cron.RestartLogrotate()
	}

	// Validate SSL certificates if HTTPS is enabled
	needReloadCert := false
	if json.Server.EnableHTTPS != cSettings.ServerSettings.EnableHTTPS {
		needReloadCert = true
	}

	if json.Server.EnableHTTPS {
		err := system.ValidateSSLCertificates(json.Server.SSLCert, json.Server.SSLKey)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	cSettings.ProtectedFill(cSettings.AppSettings, &json.App)
	cSettings.ProtectedFill(cSettings.ServerSettings, &json.Server)
	cSettings.ProtectedFill(settings.AuthSettings, &json.Auth)
	cSettings.ProtectedFill(settings.CertSettings, &json.Cert)
	cSettings.ProtectedFill(settings.HTTPSettings, &json.Http)
	cSettings.ProtectedFill(settings.NodeSettings, &json.Node)
	cSettings.ProtectedFill(settings.OpenAISettings, &json.Openai)
	cSettings.ProtectedFill(settings.LogrotateSettings, &json.Logrotate)
	cSettings.ProtectedFill(settings.NginxSettings, &json.Nginx)

	err := settings.Save()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if needReloadCert {
		go func() {
			cert.ReloadServerTLSCertificate()
		}()
	}

	GetSettings(c)
}
