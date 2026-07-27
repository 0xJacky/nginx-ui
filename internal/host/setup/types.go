package setup

import (
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

// SetupParams is the single shared input model for all template renders,
// the verify pipeline, and the CLI/Web UI surfaces.
type SetupParams struct {
	// Host-side connectivity
	HostAddress    string `json:"host_address"` // "host.docker.internal:22" | "192.168.x.x:22"
	HostUser       string `json:"host_user"`
	UseHostGateway bool   `json:"use_host_gateway,omitempty"` // derived: true when HostAddress starts with "host.docker.internal"
	ServiceManager string `json:"service_manager,omitempty"`  // "systemd" | "launchd"
	SystemdUnit    string `json:"systemd_unit,omitempty"`     // e.g. "nginx.service"
	SystemctlPath  string `json:"systemctl_path,omitempty"`   // e.g. "/bin/systemctl"
	LaunchdService string `json:"launchd_service,omitempty"`  // e.g. "homebrew.mxcl.nginx"
	LaunchctlPath  string `json:"launchctl_path,omitempty"`   // e.g. "/bin/launchctl"
	NginxSbinPath  string `json:"nginx_sbin_path,omitempty"`
	HostConfigDir  string `json:"host_config_dir,omitempty"`
	HostLogDir     string `json:"host_log_dir,omitempty"`
	PIDPath        string `json:"pid_path,omitempty"`
	PIDDir         string `json:"-"`

	// User-managed key paths (only when UseGeneratedKey==false)
	HostKeyPath        string `json:"host_key_path,omitempty"`
	HostKnownHostsPath string `json:"host_known_hosts_path,omitempty"`

	// Container-side paths (default mirrors HostConfigDir/HostLogDir)
	ContainerConfigDir      string `json:"container_config_dir,omitempty"`
	ContainerLogDir         string `json:"container_log_dir,omitempty"`
	ContainerKeyPath        string `json:"container_key_path,omitempty"`
	ContainerKnownHostsPath string `json:"container_known_hosts_path,omitempty"`

	// Key handling
	UseGeneratedKey  bool   `json:"use_generated_key,omitempty"`
	PublicKeyOpenSSH string `json:"public_key_open_ssh,omitempty"` // single-line OpenSSH-formatted public key

	// File names
	SudoersFilename string `json:"sudoers_filename,omitempty"`
}

// FillDefaults returns p with empty fields populated by sensible defaults.
// Caller-supplied values are never overwritten.
func (p SetupParams) FillDefaults() SetupParams {
	if p.ServiceManager == "" {
		p.ServiceManager = settings.HostServiceManagerSystemd
	}
	p.UseHostGateway = p.UseHostGateway || strings.HasPrefix(p.HostAddress, "host.docker.internal")
	if p.ServiceManager == settings.HostServiceManagerLaunchd {
		if p.LaunchdService == "" {
			p.LaunchdService = "homebrew.mxcl.nginx"
		}
		if p.LaunchctlPath == "" {
			p.LaunchctlPath = "/bin/launchctl"
		}
		if p.NginxSbinPath == "" {
			p.NginxSbinPath = "/opt/homebrew/bin/nginx"
		}
		if p.HostConfigDir == "" {
			p.HostConfigDir = "/opt/homebrew/etc/nginx"
		}
		if p.HostLogDir == "" {
			p.HostLogDir = "/opt/homebrew/var/log/nginx"
		}
		if p.PIDPath == "" {
			p.PIDPath = "/opt/homebrew/var/run/nginx.pid"
		}
	} else {
		p.ServiceManager = settings.HostServiceManagerSystemd
		if p.NginxSbinPath == "" {
			p.NginxSbinPath = "/usr/sbin/nginx"
		}
		if p.HostConfigDir == "" {
			p.HostConfigDir = "/etc/nginx"
		}
		if p.HostLogDir == "" {
			p.HostLogDir = "/var/log/nginx"
		}
		if p.PIDPath == "" {
			p.PIDPath = "/var/run/nginx.pid"
		}
	}
	if p.SystemdUnit == "" {
		p.SystemdUnit = "nginx.service"
	}
	if p.SystemctlPath == "" {
		p.SystemctlPath = "/bin/systemctl"
	}
	p.PIDDir = filepath.Dir(p.PIDPath)
	if p.ContainerConfigDir == "" {
		p.ContainerConfigDir = p.HostConfigDir
	}
	if p.ContainerLogDir == "" {
		p.ContainerLogDir = p.HostLogDir
	}
	if p.ContainerKeyPath == "" {
		p.ContainerKeyPath = "/etc/nginx-ui/host_key"
	}
	if p.ContainerKnownHostsPath == "" {
		p.ContainerKnownHostsPath = "/etc/nginx-ui/known_hosts"
	}
	if p.SudoersFilename == "" {
		p.SudoersFilename = "/etc/sudoers.d/nginx-ui"
	}
	return p
}

func (p SetupParams) IsLaunchd() bool {
	return p.ServiceManager == settings.HostServiceManagerLaunchd
}
