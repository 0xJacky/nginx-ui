package setup

// SetupParams is the single shared input model for all template renders,
// the verify pipeline, and the CLI/Web UI surfaces.
type SetupParams struct {
	// Host-side connectivity
	HostAddress    string // "host.docker.internal:22" | "192.168.x.x:22"
	HostUser       string
	UseHostGateway bool   // derived: true when HostAddress starts with "host.docker.internal"
	SystemdUnit    string // e.g. "nginx.service"
	SystemctlPath  string // discovered, e.g. "/bin/systemctl"
	NginxSbinPath  string // discovered, e.g. "/usr/sbin/nginx"
	HostConfigDir  string // e.g. "/etc/nginx"
	HostLogDir     string // e.g. "/var/log/nginx"

	// User-managed key paths (only when UseGeneratedKey==false)
	HostKeyPath        string
	HostKnownHostsPath string

	// Container-side paths (default mirrors HostConfigDir/HostLogDir)
	ContainerConfigDir      string
	ContainerLogDir         string
	ContainerKeyPath        string
	ContainerKnownHostsPath string

	// Key handling
	UseGeneratedKey  bool
	PublicKeyOpenSSH string // single-line OpenSSH-formatted public key

	// File names
	SudoersFilename string
}

// FillDefaults returns p with empty fields populated by sensible defaults.
// Caller-supplied values are never overwritten.
func (p SetupParams) FillDefaults() SetupParams {
	if p.SystemdUnit == "" {
		p.SystemdUnit = "nginx.service"
	}
	if p.SystemctlPath == "" {
		p.SystemctlPath = "/bin/systemctl"
	}
	if p.NginxSbinPath == "" {
		p.NginxSbinPath = "/usr/sbin/nginx"
	}
	if p.HostConfigDir == "" {
		p.HostConfigDir = "/etc/nginx"
	}
	if p.HostLogDir == "" {
		p.HostLogDir = "/var/log/nginx"
	}
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
