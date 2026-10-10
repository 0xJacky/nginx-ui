package settings

import (
	"net/http"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/process"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// settingsSection describes one group of settings that can be saved on its
// own. Every group is bound, validated and persisted independently, so a
// validation failure is reported against the group and field it belongs to
// instead of failing an unrelated part of the preference page.
type settingsSection[T any] struct {
	name string
	// target returns the live settings struct the payload is written into.
	target func() *T
	// restore swaps redacted placeholders echoed back by the client for the
	// stored values.
	restore func(payload *T)
	// validate runs checks that the binding tags cannot express.
	validate func(payload *T) error
	// afterSave applies the side effects of a change once it is persisted.
	afterSave func(previous, current T)
}

// sectionSave is a bound save of one section: the payload has been decoded and
// the steps below run in order around the settings write.
type sectionSave struct {
	validate  func() error
	apply     func()
	afterSave func()
	mfaPolicy *settings.Auth
}

func (s settingsSection[T]) bind(payload *T) *sectionSave {
	var previous T
	target := s.target()

	save := &sectionSave{
		validate: func() error {
			if s.restore != nil {
				s.restore(payload)
			}
			if s.validate != nil {
				return s.validate(payload)
			}
			return nil
		},
		apply: func() {
			previous = *target
			cSettings.ProtectedFill(target, payload)
		},
		afterSave: func() {
			if s.afterSave != nil {
				s.afterSave(previous, *target)
			}
		},
	}
	if s.name == "auth" {
		save.mfaPolicy = any(payload).(*settings.Auth)
	}
	return save
}

// handler serves POST /settings/<name>: it saves this section only and answers
// with the section as GET /settings would return it.
func (s settingsSection[T]) handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload T
		if s.name == "auth" {
			*any(&payload).(*settings.Auth) = *settings.AuthSettings
		}
		if !cosy.BindAndValid(c, &payload) {
			return
		}
		if s.name == "auth" && !authorizeMFAPolicy(c, any(&payload).(*settings.Auth)) {
			return
		}

		save := s.bind(&payload)
		if !persistSections(c, save) {
			return
		}

		c.JSON(http.StatusOK, buildSettingsSectionResponse(s.name))
		save.afterSave()
	}
}

func authorizeMFAPolicy(c *gin.Context, payload *settings.Auth) bool {
	if payload.MFARequired == settings.AuthSettings.MFARequired && payload.MFARequiredForSSO == settings.AuthSettings.MFARequiredForSSO {
		return true
	}
	return middleware.RequireMFAManagement(c)
}

// persistSections validates every section first and then writes them in a
// single settings update, so a combined save either stores all of them or
// none. Side effects are left to the caller, which runs them once the
// response is written.
func persistSections(c *gin.Context, saves ...*sectionSave) bool {
	for _, save := range saves {
		if err := save.validate(); err != nil {
			abortBadRequest(c, err)
			return false
		}
	}

	authorized := true
	err := settings.Update(func() {
		// Recheck against the current policy while holding the settings lock.
		for _, save := range saves {
			if save.mfaPolicy != nil && !authorizeMFAPolicy(c, save.mfaPolicy) {
				authorized = false
				return
			}
		}
		for _, save := range saves {
			save.apply()
		}
	})
	if !authorized {
		return false
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return false
	}

	return true
}

var appSection = settingsSection[cSettings.App]{
	name:   "app",
	target: func() *cSettings.App { return cSettings.AppSettings },
	restore: func(payload *cSettings.App) {
		if payload.JwtSecret == redactedSensitiveValue {
			payload.JwtSecret = cSettings.AppSettings.JwtSecret
		}
	},
}

var serverSection = settingsSection[cSettings.Server]{
	name:     "server",
	target:   func() *cSettings.Server { return cSettings.ServerSettings },
	validate: validateServerSettings,
	afterSave: func(previous, current cSettings.Server) {
		httpsChanged := previous.EnableHTTPS != current.EnableHTTPS
		if httpsChanged || previous.SSLCert != current.SSLCert || previous.SSLKey != current.SSLKey {
			go cert.ReloadServerTLSCertificate()
		}
		if httpsChanged {
			go risefront.Restart()
		}
	},
}

func validateServerSettings(payload *cSettings.Server) error {
	if payload.EnableHTTPS {
		if err := system.ValidateSSLCertificates(payload.SSLCert, payload.SSLKey); err != nil {
			return err
		}
	}

	if payload.EnableH2 && !payload.EnableHTTPS {
		return ErrHTTP2RequiresHTTPS
	}

	if payload.EnableH3 && !payload.EnableHTTPS {
		return ErrHTTP3RequiresHTTPS
	}

	// HTTP/3 needs a UDP listener, which a Unix socket cannot provide. The
	// startup validation would otherwise refuse to boot after the restart
	// this save triggers, taking the UI down until app.ini is edited by hand.
	// Check the active listener too: after a graceful restart the running
	// program may have loaded a TCP configuration while the parent process
	// still owns a Unix socket, and the next restart re-reads app.ini.
	activeNetwork, _ := process.ActiveListener()
	if payload.EnableH3 && (settings.ListenerSettings.UnixSocket != "" || activeNetwork == "unix") {
		return ErrHTTP3OnUnixSocket
	}

	return nil
}

var authSection = settingsSection[settings.Auth]{
	name:   "auth",
	target: func() *settings.Auth { return settings.AuthSettings },
}

var certSection = settingsSection[settings.Cert]{
	name:   "cert",
	target: func() *settings.Cert { return settings.CertSettings },
}

var httpSection = settingsSection[settings.HTTP]{
	name:   "http",
	target: func() *settings.HTTP { return settings.HTTPSettings },
}

var nodeSection = settingsSection[settings.Node]{
	name:   "node",
	target: func() *settings.Node { return settings.NodeSettings },
	restore: func(payload *settings.Node) {
		if payload.Secret == redactedSensitiveValue {
			payload.Secret = settings.NodeSettings.Secret
		}
	},
}

var openAISection = settingsSection[settings.OpenAI]{
	name:   "openai",
	target: func() *settings.OpenAI { return settings.OpenAISettings },
	restore: func(payload *settings.OpenAI) {
		if payload.Token == redactedSensitiveValue {
			payload.Token = settings.OpenAISettings.Token
		}
	},
}

var logrotateSection = settingsSection[settings.Logrotate]{
	name:   "logrotate",
	target: func() *settings.Logrotate { return settings.LogrotateSettings },
	afterSave: func(previous, current settings.Logrotate) {
		if previous.Enabled != current.Enabled || previous.Interval != current.Interval {
			go cron.RestartLogrotate()
		}
	},
}

var nginxSection = settingsSection[settings.Nginx]{
	name:   "nginx",
	target: func() *settings.Nginx { return settings.NginxSettings },
	afterSave: func(settings.Nginx, settings.Nginx) {
		// Host SSH settings may have changed, so drop the cached SSH client
		// and let the next nginx command re-dial with the new config.
		nginx.ResetHostNginxState()
	},
}

var oidcSection = settingsSection[settings.OIDC]{
	name:   "oidc",
	target: func() *settings.OIDC { return settings.OIDCSettings },
}

var siteCheckSection = settingsSection[settings.SiteCheck]{
	name:   "site_check",
	target: func() *settings.SiteCheck { return settings.SiteCheckSettings },
	afterSave: func(previous, current settings.SiteCheck) {
		if previous == current {
			return
		}
		if service := sitecheck.GetService(); service != nil {
			service.SettingsChanged()
		}
	},
}

var upstreamCheckSection = settingsSection[settings.UpstreamCheck]{
	name:   "upstream_check",
	target: func() *settings.UpstreamCheck { return settings.UpstreamCheckSettings },
	afterSave: func(previous, current settings.UpstreamCheck) {
		if previous == current {
			return
		}
		go func() {
			if err := cron.RestartUpstreamAvailabilityJob(); err != nil {
				// The settings have already been saved. Surface restart failures in
				// server logs so the next scheduled reload can recover.
				logger.Errorf("Failed to restart upstream availability job: %v", err)
			}
		}()
	},
}

// sectionHandlers maps each writable section to its save handler. The keys
// match the section names in the GET /settings response.
func sectionHandlers() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		appSection.name:           appSection.handler(),
		serverSection.name:        serverSection.handler(),
		authSection.name:          authSection.handler(),
		certSection.name:          certSection.handler(),
		httpSection.name:          httpSection.handler(),
		nodeSection.name:          nodeSection.handler(),
		openAISection.name:        openAISection.handler(),
		logrotateSection.name:     logrotateSection.handler(),
		nginxSection.name:         nginxSection.handler(),
		oidcSection.name:          oidcSection.handler(),
		siteCheckSection.name:     siteCheckSection.handler(),
		upstreamCheckSection.name: upstreamCheckSection.handler(),
	}
}

var sectionHandlerMap = sectionHandlers()

// SaveSettingsSection saves a single settings group, named by the :section
// path parameter.
func SaveSettingsSection(c *gin.Context) {
	name := c.Param("section")
	handler, ok := sectionHandlerMap[name]
	if !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, cosy.WrapErrorWithParams(ErrUnknownSettingsSection, name))
		return
	}

	handler(c)
}
