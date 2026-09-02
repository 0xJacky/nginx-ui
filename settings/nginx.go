package settings

const (
	ControlModeLocal             = "local"
	ControlModeExternalContainer = "external_container"
	ControlModeHostViaSSH        = "host_via_ssh"

	HostModeSSH           = "ssh"
	HostAccessModeMounted = "mounted"
	HostAccessModeSFTP    = "sftp"

	HostServiceManagerSystemd = "systemd"
	HostServiceManagerLaunchd = "launchd"
	HostKeySourceGenerated    = "generated"
	HostKeySourceExisting     = "existing"
	HostKeySourceProvided     = "provided"

	DefaultHostPrivateKeyPath = "/etc/nginx-ui/host_key"
	DefaultHostKnownHostsPath = "/etc/nginx-ui/known_hosts"

	// Default nginx binaries on an SSH host, per service manager. The setup
	// wizard verifies against these, so runtime control must resolve to the
	// same paths when SbinPath is left empty.
	DefaultHostSbinPathSystemd = "/usr/sbin/nginx"
	DefaultHostSbinPathLaunchd = "/opt/homebrew/opt/nginx/bin/nginx"
)

const DefaultMaintenanceDir = "/etc/nginx/maintenance"

type Nginx struct {
	AccessLogPath       string   `json:"access_log_path" protected:"true"`
	ErrorLogPath        string   `json:"error_log_path" protected:"true"`
	LogDirWhiteList     []string `json:"log_dir_white_list" protected:"true"`
	ConfigDir           string   `json:"config_dir" protected:"true"`
	ConfigPath          string   `json:"config_path" protected:"true"`
	PIDPath             string   `json:"pid_path" protected:"true"`
	SbinPath            string   `json:"sbin_path" protected:"true"`
	TestConfigCmd       string   `json:"test_config_cmd" protected:"true"`
	ReloadCmd           string   `json:"reload_cmd" protected:"true"`
	RestartCmd          string   `json:"restart_cmd" protected:"true"`
	StubStatusPort      uint     `json:"stub_status_port" binding:"omitempty,min=1,max=65535"`
	ContainerName       string   `json:"container_name" protected:"true"`
	MaintenanceDir      string   `json:"maintenance_dir" protected:"true"`
	MaintenanceTemplate string   `json:"maintenance_template"`

	// Host SSH mode fields enable nginx-ui (running in Docker) to control
	// nginx installed natively on the same host via an SSH tunnel.
	HostMode            string `json:"host_mode" protected:"true"`
	HostAccessMode      string `json:"host_access_mode" protected:"true"`
	HostAddress         string `json:"host_address" protected:"true"`
	HostUser            string `json:"host_user" protected:"true"`
	HostAuthMethod      string `json:"host_auth_method" protected:"true"`
	HostKeySource       string `json:"host_key_source" protected:"true"`
	HostPrivateKeyPath  string `json:"host_private_key_path" protected:"true"`
	HostPasswordRef     string `json:"host_password_ref" protected:"true"`
	HostKnownHostsPath  string `json:"host_known_hosts_path" protected:"true"`
	HostSudoPrefix      string `json:"host_sudo_prefix" protected:"true"`
	HostServiceManager  string `json:"host_service_manager" protected:"true"`
	HostSystemdUnitName string `json:"host_systemd_unit_name" protected:"true"`
	HostSystemctlPath   string `json:"host_systemctl_path" protected:"true"`
	HostLaunchdService  string `json:"host_launchd_service" protected:"true"`
	HostLaunchctlPath   string `json:"host_launchctl_path" protected:"true"`
	HostConfigDir       string `json:"host_config_dir" protected:"true"`
	HostLogDir          string `json:"host_log_dir" protected:"true"`
}

var NginxSettings = &Nginx{}

func (n *Nginx) GetStubStatusPort() uint {
	if n.StubStatusPort == 0 {
		return 51820
	}
	return n.StubStatusPort
}

func (n *Nginx) GetMaintenanceDir() string {
	if n.MaintenanceDir == "" {
		return DefaultMaintenanceDir
	}
	return n.MaintenanceDir
}

func (n *Nginx) GetHostKnownHostsPath() string {
	if n.HostKnownHostsPath == "" {
		return DefaultHostKnownHostsPath
	}
	return n.HostKnownHostsPath
}

func (n *Nginx) GetHostPrivateKeyPath() string {
	if n.HostPrivateKeyPath == "" {
		return DefaultHostPrivateKeyPath
	}
	return n.HostPrivateKeyPath
}

func (n *Nginx) GetHostKeySource() string {
	if n.HostKeySource == HostKeySourceGenerated ||
		n.HostKeySource == HostKeySourceExisting ||
		n.HostKeySource == HostKeySourceProvided {
		return n.HostKeySource
	}
	if n.GetHostPrivateKeyPath() == DefaultHostPrivateKeyPath {
		return HostKeySourceGenerated
	}
	return HostKeySourceExisting
}

func (n *Nginx) GetHostServiceManager() string {
	if n.HostServiceManager == HostServiceManagerLaunchd {
		return HostServiceManagerLaunchd
	}
	return HostServiceManagerSystemd
}

// GetHostSbinPath returns the nginx binary to run on the SSH host: the
// configured SbinPath, or the service manager's default when it is empty.
// A container-local lookup would name a binary the host may not have and
// would never match the sudo whitelist.
func (n *Nginx) GetHostSbinPath() string {
	if n.SbinPath != "" {
		return n.SbinPath
	}
	if n.GetHostServiceManager() == HostServiceManagerLaunchd {
		return DefaultHostSbinPathLaunchd
	}
	return DefaultHostSbinPathSystemd
}

func (n *Nginx) UsesSFTP() bool {
	return n.ControlMode() == ControlModeHostViaSSH && n.HostAccessMode == HostAccessModeSFTP
}

func (n *Nginx) GetHostSudoPrefix() string {
	if n.GetHostServiceManager() == HostServiceManagerLaunchd {
		return ""
	}
	if n.HostSudoPrefix == "" {
		return "sudo -n"
	}
	return n.HostSudoPrefix
}

func (n *Nginx) GetHostLaunchdService() string {
	if n.HostLaunchdService == "" {
		return "homebrew.mxcl.nginx"
	}
	return n.HostLaunchdService
}

func (n *Nginx) GetHostLaunchctlPath() string {
	if n.HostLaunchctlPath == "" {
		return "/bin/launchctl"
	}
	return n.HostLaunchctlPath
}

// RunningInAnotherContainer reports whether nginx-ui should control nginx
// in a separate Docker container via the docker.sock channel.
// This stays semantically narrow: SSH mode does NOT count.
func (n *Nginx) RunningInAnotherContainer() bool {
	return n.ControlMode() == ControlModeExternalContainer
}

// ControlMode resolves which nginx control channel to use.
// Priority: host_via_ssh > external_container > local.
func (n *Nginx) ControlMode() string {
	if n.HostMode == HostModeSSH {
		return ControlModeHostViaSSH
	}
	if n.ContainerName != "" {
		return ControlModeExternalContainer
	}
	return ControlModeLocal
}
