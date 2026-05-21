package settings

const (
	ControlModeLocal             = "local"
	ControlModeExternalContainer = "external_container"
	ControlModeHostViaSSH        = "host_via_ssh"

	HostModeSSH = "ssh"
)

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
	MaintenanceTemplate string   `json:"maintenance_template"`

	// Host SSH mode fields enable nginx-ui (running in Docker) to control
	// nginx installed natively on the same host via an SSH tunnel.
	HostMode            string `json:"host_mode" protected:"true"`
	HostAddress         string `json:"host_address" protected:"true"`
	HostUser            string `json:"host_user" protected:"true"`
	HostAuthMethod      string `json:"host_auth_method" protected:"true"`
	HostPrivateKeyPath  string `json:"host_private_key_path" protected:"true"`
	HostPasswordRef     string `json:"host_password_ref" protected:"true"`
	HostKnownHostsPath  string `json:"host_known_hosts_path" protected:"true"`
	HostSudoPrefix      string `json:"host_sudo_prefix" protected:"true"`
	HostSystemdUnitName string `json:"host_systemd_unit_name" protected:"true"`
	HostSystemctlPath   string `json:"host_systemctl_path" protected:"true"`
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
