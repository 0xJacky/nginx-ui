package setup

import (
	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
)

// NewClientFromSettings constructs a hostssh.Client using the currently loaded
// settings.NginxSettings. The returned client is single-use for verify flows;
// the long-lived client used by sshRunner is independent.
func NewClientFromSettings() (*hostssh.Client, error) {
	return NewClientFromParams(ParamsFromSettings())
}

// NewClientFromParams constructs a client for wizard verification without
// persisting the in-progress host and service-manager settings.
func NewClientFromParams(params SetupParams) (*hostssh.Client, error) {
	n := settings.NginxSettings
	p := params.FillDefaults()
	kh, err := hostssh.NewKnownHosts(n.GetHostKnownHostsPath())
	if err != nil {
		return nil, err
	}
	sudo := n.HostSudoPrefix
	if p.IsLaunchd() {
		sudo = ""
	} else if sudo == "" {
		sudo = "sudo -n"
	}
	return hostssh.NewClient(hostssh.ClientOptions{
		Address:        p.HostAddress,
		User:           p.HostUser,
		AuthMethod:     n.HostAuthMethod,
		PrivateKeyPath: n.HostPrivateKeyPath,
		KnownHosts:     kh,
		Config: hostssh.Config{
			SudoPrefix:    sudo,
			SystemctlPath: p.SystemctlPath,
			NginxSbinPath: p.NginxSbinPath,
		},
	}), nil
}

// ParamsFromSettings builds a SetupParams reflecting current settings.
func ParamsFromSettings() SetupParams {
	n := settings.NginxSettings
	p := SetupParams{
		HostAddress:             n.HostAddress,
		HostUser:                n.HostUser,
		ServiceManager:          n.GetHostServiceManager(),
		SystemdUnit:             n.HostSystemdUnitName,
		SystemctlPath:           n.HostSystemctlPath,
		LaunchdService:          n.GetHostLaunchdService(),
		LaunchctlPath:           n.GetHostLaunchctlPath(),
		NginxSbinPath:           n.SbinPath,
		HostConfigDir:           n.HostConfigDir,
		HostLogDir:              n.HostLogDir,
		PIDPath:                 n.PIDPath,
		ContainerKeyPath:        n.HostPrivateKeyPath,
		ContainerKnownHostsPath: n.GetHostKnownHostsPath(),
	}
	return p.FillDefaults()
}
