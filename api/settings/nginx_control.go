package settings

import (
	"errors"
	"net/http"
	"strings"

	hostsetup "github.com/0xJacky/Nginx-UI/internal/host/setup"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

const maxPastedPrivateKeySize = 64 * 1024

var saveManagedHostPrivateKey = hostsetup.SavePrivateKey

type nginxControlSettingsPayload struct {
	Mode                string `json:"mode" binding:"required,oneof=local external_container host_via_ssh"`
	ContainerName       string `json:"container_name"`
	HostAddress         string `json:"host_address"`
	HostUser            string `json:"host_user"`
	HostAccessMode      string `json:"host_access_mode"`
	HostAuthMethod      string `json:"host_auth_method"`
	HostKeySource       string `json:"host_key_source"`
	HostPrivateKeyPath  string `json:"host_private_key_path"`
	HostPasswordRef     string `json:"host_password_ref"`
	HostKnownHostsPath  string `json:"host_known_hosts_path"`
	HostSudoPrefix      string `json:"host_sudo_prefix"`
	HostServiceManager  string `json:"host_service_manager"`
	HostSystemdUnitName string `json:"host_systemd_unit_name"`
	HostSystemctlPath   string `json:"host_systemctl_path"`
	HostLaunchdService  string `json:"host_launchd_service"`
	HostLaunchctlPath   string `json:"host_launchctl_path"`
	HostConfigDir       string `json:"host_config_dir"`
	HostLogDir          string `json:"host_log_dir"`
	SbinPath            string `json:"sbin_path"`
	PIDPath             string `json:"pid_path"`
	ConfigDir           string `json:"config_dir"`
	ConfigPath          string `json:"config_path"`
	AccessLogPath       string `json:"access_log_path"`
	ErrorLogPath        string `json:"error_log_path"`
}

func normalizeNginxControlSettings(payload nginxControlSettingsPayload) nginxControlSettingsPayload {
	payload.Mode = strings.TrimSpace(payload.Mode)
	payload.ContainerName = strings.TrimSpace(payload.ContainerName)
	payload.HostAddress = strings.TrimSpace(payload.HostAddress)
	payload.HostUser = strings.TrimSpace(payload.HostUser)
	payload.HostAccessMode = strings.TrimSpace(payload.HostAccessMode)
	payload.HostAuthMethod = strings.TrimSpace(payload.HostAuthMethod)
	payload.HostKeySource = strings.TrimSpace(payload.HostKeySource)
	payload.HostPrivateKeyPath = strings.TrimSpace(payload.HostPrivateKeyPath)
	if payload.HostKeySource == "" {
		if payload.HostPrivateKeyPath == appsettings.DefaultHostPrivateKeyPath {
			payload.HostKeySource = appsettings.HostKeySourceGenerated
		} else {
			payload.HostKeySource = appsettings.HostKeySourceExisting
		}
	}
	payload.HostPasswordRef = strings.TrimSpace(payload.HostPasswordRef)
	payload.HostKnownHostsPath = strings.TrimSpace(payload.HostKnownHostsPath)
	payload.HostSudoPrefix = strings.TrimSpace(payload.HostSudoPrefix)
	payload.HostServiceManager = strings.TrimSpace(payload.HostServiceManager)
	payload.HostSystemdUnitName = strings.TrimSpace(payload.HostSystemdUnitName)
	payload.HostSystemctlPath = strings.TrimSpace(payload.HostSystemctlPath)
	payload.HostLaunchdService = strings.TrimSpace(payload.HostLaunchdService)
	payload.HostLaunchctlPath = strings.TrimSpace(payload.HostLaunchctlPath)
	payload.HostConfigDir = strings.TrimSpace(payload.HostConfigDir)
	payload.HostLogDir = strings.TrimSpace(payload.HostLogDir)
	payload.SbinPath = strings.TrimSpace(payload.SbinPath)
	payload.PIDPath = strings.TrimSpace(payload.PIDPath)
	payload.ConfigDir = strings.TrimSpace(payload.ConfigDir)
	payload.ConfigPath = strings.TrimSpace(payload.ConfigPath)
	payload.AccessLogPath = strings.TrimSpace(payload.AccessLogPath)
	payload.ErrorLogPath = strings.TrimSpace(payload.ErrorLogPath)
	return payload
}

// controlSettings extracts the mode-dependent fields the validator inspects.
func controlSettings(payload nginxControlSettingsPayload) hostsetup.ControlSettings {
	return hostsetup.ControlSettings{
		Mode:               payload.Mode,
		ContainerName:      payload.ContainerName,
		HostAddress:        payload.HostAddress,
		HostUser:           payload.HostUser,
		HostAccessMode:     payload.HostAccessMode,
		HostAuthMethod:     payload.HostAuthMethod,
		HostKeySource:      payload.HostKeySource,
		HostPrivateKeyPath: payload.HostPrivateKeyPath,
		HostKnownHostsPath: payload.HostKnownHostsPath,
		HostServiceManager: payload.HostServiceManager,
	}
}

func validateNginxControlSettings(payload nginxControlSettingsPayload) error {
	return hostsetup.ValidateControlSettings(controlSettings(payload))
}

// abortBadRequest answers a validation failure with 400 while keeping the
// cosy scope and code so the frontend can map the message to a translation.
// cosy.ErrHandler would report the same body as 500, which is wrong for
// input the operator can correct.
func abortBadRequest(c *gin.Context, err error) {
	var cErr *cosy.Error
	if errors.As(err, &cErr) {
		c.AbortWithStatusJSON(http.StatusBadRequest, cErr)
		return
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
}

func applyNginxControlSettings(target *appsettings.Nginx, payload nginxControlSettingsPayload) {
	wasHostViaSSH := target.HostMode == appsettings.HostModeSSH
	target.HostMode = ""
	target.ContainerName = ""

	switch payload.Mode {
	case appsettings.ControlModeExternalContainer:
		target.ContainerName = payload.ContainerName
	case appsettings.ControlModeHostViaSSH:
		target.HostMode = appsettings.HostModeSSH
		target.HostAccessMode = payload.HostAccessMode
		target.HostAddress = payload.HostAddress
		target.HostUser = payload.HostUser
		target.HostAuthMethod = payload.HostAuthMethod
		target.HostKeySource = payload.HostKeySource
		target.HostPrivateKeyPath = payload.HostPrivateKeyPath
		target.HostPasswordRef = payload.HostPasswordRef
		target.HostKnownHostsPath = payload.HostKnownHostsPath
		target.HostSudoPrefix = payload.HostSudoPrefix
		target.HostServiceManager = payload.HostServiceManager
		target.HostSystemdUnitName = payload.HostSystemdUnitName
		target.HostSystemctlPath = payload.HostSystemctlPath
		target.HostLaunchdService = payload.HostLaunchdService
		target.HostLaunchctlPath = payload.HostLaunchctlPath
		target.HostConfigDir = payload.HostConfigDir
		target.HostLogDir = payload.HostLogDir
		target.SbinPath = payload.SbinPath
		if target.SbinPath == "" {
			// The wizard always sends the path it verified, but the plain
			// settings form may not. Persist the same default the verifier
			// used so runtime control never falls back to a container lookup.
			target.SbinPath = target.GetHostSbinPath()
		}
		target.PIDPath = payload.PIDPath
		target.ConfigDir = payload.ConfigDir
		target.ConfigPath = payload.ConfigPath
		target.AccessLogPath = payload.AccessLogPath
		target.ErrorLogPath = payload.ErrorLogPath
	}

	// TestConfigCmd, ReloadCmd and RestartCmd are deliberately left alone here.
	// Nothing in the wizard writes them, so they are operator authored, and
	// execShell already routes them at the current target. Clearing them on a
	// mode switch would silently discard input the operator typed elsewhere.
	//
	// Paths detected on the SSH host do not describe a local or docker install.
	// Clear them on the way out so path resolution falls back to detection.
	if wasHostViaSSH && payload.Mode != appsettings.ControlModeHostViaSSH {
		target.SbinPath = ""
		target.PIDPath = ""
		target.ConfigDir = ""
		target.ConfigPath = ""
		target.AccessLogPath = ""
		target.ErrorLogPath = ""
		target.HostConfigDir = ""
		target.HostLogDir = ""
	}
}

func currentNginxControlSettings() nginxControlSettingsPayload {
	n := appsettings.NginxSettings
	return nginxControlSettingsPayload{
		Mode:                n.ControlMode(),
		ContainerName:       n.ContainerName,
		HostAddress:         n.HostAddress,
		HostUser:            n.HostUser,
		HostAccessMode:      n.HostAccessMode,
		HostAuthMethod:      n.HostAuthMethod,
		HostKeySource:       n.GetHostKeySource(),
		HostPrivateKeyPath:  n.HostPrivateKeyPath,
		HostPasswordRef:     n.HostPasswordRef,
		HostKnownHostsPath:  n.HostKnownHostsPath,
		HostSudoPrefix:      n.HostSudoPrefix,
		HostServiceManager:  n.HostServiceManager,
		HostSystemdUnitName: n.HostSystemdUnitName,
		HostSystemctlPath:   n.HostSystemctlPath,
		HostLaunchdService:  n.HostLaunchdService,
		HostLaunchctlPath:   n.HostLaunchctlPath,
		HostConfigDir:       n.HostConfigDir,
		HostLogDir:          n.HostLogDir,
		SbinPath:            n.SbinPath,
		// GET /settings reports the resolved values for these, and the frontend
		// merges this response into the same store without refetching, so the
		// two must agree or a save would blank the displayed paths.
		PIDPath:       nginx.GetPIDPath(),
		ConfigDir:     nginx.GetConfPath(),
		ConfigPath:    n.ConfigPath,
		AccessLogPath: nginx.GetAccessLogPath(),
		ErrorLogPath:  nginx.GetErrorLogPath(),
	}
}

var updateNginxControlSettings = func(payload nginxControlSettingsPayload) error {
	return appsettings.Update(func() {
		applyNginxControlSettings(appsettings.NginxSettings, payload)
	})
}

func SaveNginxControlSettings(c *gin.Context) {
	if !requireVerifiedTwoFactorOrProxy(c, "Two-factor authentication is required to modify nginx control settings") {
		return
	}

	var payload nginxControlSettingsPayload
	if !cosy.BindAndValid(c, &payload) {
		return
	}
	payload = normalizeNginxControlSettings(payload)
	if err := validateNginxControlSettings(payload); err != nil {
		abortBadRequest(c, err)
		return
	}

	if err := updateNginxControlSettings(payload); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	nginx.ResetHostNginxState()
	c.JSON(http.StatusOK, currentNginxControlSettings())
}

type nginxPrivateKeyPayload struct {
	PrivateKey string `json:"private_key" binding:"required"`
}

func SaveNginxPrivateKey(c *gin.Context) {
	if !requireVerifiedTwoFactorOrProxy(c, "Two-factor authentication is required to store an SSH private key") {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPastedPrivateKeySize+1024)
	var payload nginxPrivateKeyPayload
	if !cosy.BindAndValid(c, &payload) {
		return
	}
	if len(payload.PrivateKey) > maxPastedPrivateKeySize {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "private key exceeds the 64 KiB limit"})
		return
	}
	publicKey, err := saveManagedHostPrivateKey(appsettings.DefaultHostPrivateKeyPath, []byte(payload.PrivateKey))
	if errors.Is(err, hostsetup.ErrInvalidPrivateKey) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "private key is invalid or encrypted"})
		return
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"private_key_path": appsettings.DefaultHostPrivateKeyPath,
		"public_key":       publicKey,
	})
}
