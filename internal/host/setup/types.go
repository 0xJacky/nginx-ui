package setup

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
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
	AccessMode     string `json:"access_mode,omitempty"` // "sftp" | "mounted"

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
			p.NginxSbinPath = settings.DefaultHostSbinPathLaunchd
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
			p.NginxSbinPath = settings.DefaultHostSbinPathSystemd
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
		p.ContainerKeyPath = settings.DefaultHostPrivateKeyPath
	}
	if p.ContainerKnownHostsPath == "" {
		p.ContainerKnownHostsPath = settings.DefaultHostKnownHostsPath
	}
	if p.SudoersFilename == "" {
		p.SudoersFilename = "/etc/sudoers.d/nginx-ui"
	}
	return p
}

// hostUserPattern is the portable POSIX username syntax. The value is pasted
// into sudoers and into shell commands, so anything outside it is rejected
// rather than escaped.
var hostUserPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// ValidateHostUser rejects an SSH user that cannot be safely interpolated into
// the generated host instructions.
func ValidateHostUser(user string) error {
	if !hostUserPattern.MatchString(strings.TrimSpace(user)) {
		return ErrInvalidHostUser
	}
	return nil
}

// The generated snippets are pasted into a sudoers rule and into shell commands
// the operator runs as root. sudoers has no quoting that survives a comma, so
// every interpolated value is restricted rather than escaped.
var (
	absolutePathPattern = regexp.MustCompile(`^/[A-Za-z0-9._+@\-/]*$`)
	systemdUnitPattern  = regexp.MustCompile(`^[A-Za-z0-9@._\-]+$`)
	launchdLabelPattern = regexp.MustCompile(`^[A-Za-z0-9._\-]+$`)
	hostAddressPattern  = regexp.MustCompile(`^\[?[A-Za-z0-9._:\-]+\]?(:[0-9]{1,5})?$`)
)

// ValidateHostAddress rejects an SSH target that could not be written as a
// single known_hosts entry. known_hosts treats "," as a host separator and
// "*" as a wildcard, and a newline would start a second entry, so anything
// outside the host[:port] shape is refused before it reaches the file.
func ValidateHostAddress(address string) error {
	value := strings.TrimSpace(address)
	if value == "" || !hostAddressPattern.MatchString(value) {
		return cosy.WrapErrorWithParams(ErrInvalidHostAddress, address)
	}
	return nil
}

type snippetField struct {
	name    string
	value   string
	pattern *regexp.Regexp
}

// ValidateSnippetValues rejects any value that would change the meaning of the
// generated sudoers rule or shell snippets. Empty values are left to
// FillDefaults, which only produces known-good literals.
func (p SetupParams) ValidateSnippetValues() error {
	if p.AccessMode != settings.HostAccessModeSFTP && p.AccessMode != settings.HostAccessModeMounted {
		return ErrInvalidAccessMode
	}
	if err := ValidateHostUser(p.HostUser); err != nil {
		return err
	}

	fields := []snippetField{
		{"host_address", p.HostAddress, hostAddressPattern},
		{"systemd_unit", p.SystemdUnit, systemdUnitPattern},
		{"launchd_service", p.LaunchdService, launchdLabelPattern},
		{"systemctl_path", p.SystemctlPath, absolutePathPattern},
		{"launchctl_path", p.LaunchctlPath, absolutePathPattern},
		{"nginx_sbin_path", p.NginxSbinPath, absolutePathPattern},
		{"host_config_dir", p.HostConfigDir, absolutePathPattern},
		{"host_log_dir", p.HostLogDir, absolutePathPattern},
		{"pid_path", p.PIDPath, absolutePathPattern},
		{"host_key_path", p.HostKeyPath, absolutePathPattern},
		{"host_known_hosts_path", p.HostKnownHostsPath, absolutePathPattern},
		{"container_config_dir", p.ContainerConfigDir, absolutePathPattern},
		{"container_log_dir", p.ContainerLogDir, absolutePathPattern},
		{"container_key_path", p.ContainerKeyPath, absolutePathPattern},
		{"container_known_hosts_path", p.ContainerKnownHostsPath, absolutePathPattern},
	}

	for _, field := range fields {
		value := strings.TrimSpace(field.value)
		if value == "" {
			continue
		}
		if !field.pattern.MatchString(value) {
			return cosy.WrapErrorWithParams(ErrUnsafeSnippetValue, field.name, value)
		}
	}

	return validatePublicKeyLine(p.PublicKeyOpenSSH)
}

// validatePublicKeyLine keeps a newline out of authorized_keys, where a second
// line would install another key or a forced command.
func validatePublicKeyLine(key string) error {
	value := strings.TrimSpace(key)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return ErrInvalidPublicKey
	}
	if _, _, _, rest, err := gossh.ParseAuthorizedKey([]byte(value)); err != nil || len(bytes.TrimSpace(rest)) > 0 {
		return ErrInvalidPublicKey
	}
	return nil
}

func (p SetupParams) IsLaunchd() bool {
	return p.ServiceManager == settings.HostServiceManagerLaunchd
}

func (p SetupParams) UsesMountedFilesystem() bool {
	return p.AccessMode == settings.HostAccessModeMounted
}

// NeedsHostGatewayMapping is false on macOS runtimes, where Docker Desktop and
// OrbStack provide host.docker.internal without an extra_hosts override.
func (p SetupParams) NeedsHostGatewayMapping() bool {
	return p.UseHostGateway && !p.IsLaunchd()
}

// NeedsSudoers reports whether the generated instructions have to include a
// sudoers entry. A launchd service runs in the SSH user's own domain, and root
// already holds every privilege, so neither case needs one.
func (p SetupParams) NeedsSudoers() bool {
	return !p.IsLaunchd() && strings.TrimSpace(p.HostUser) != "root"
}
