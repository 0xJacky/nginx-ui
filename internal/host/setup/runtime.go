package setup

import (
	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
)

// NewClientFromSettings constructs a hostssh.Client using the currently loaded
// settings.NginxSettings. The returned client is single-use for verify flows;
// the long-lived client used by sshRunner is independent.
func NewClientFromSettings() (*hostssh.Client, error) {
	n := settings.NginxSettings
	kh, err := hostssh.NewKnownHosts(n.HostKnownHostsPath)
	if err != nil {
		return nil, err
	}
	sudo := n.HostSudoPrefix
	if sudo == "" {
		sudo = "sudo -n"
	}
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}
	return hostssh.NewClient(hostssh.ClientOptions{
		Address:        n.HostAddress,
		User:           n.HostUser,
		AuthMethod:     n.HostAuthMethod,
		PrivateKeyPath: n.HostPrivateKeyPath,
		KnownHosts:     kh,
		Config: hostssh.Config{
			SudoPrefix:    sudo,
			SystemctlPath: systemctl,
			NginxSbinPath: n.SbinPath,
		},
	}), nil
}

// ParamsFromSettings builds a SetupParams reflecting current settings.
func ParamsFromSettings() SetupParams {
	n := settings.NginxSettings
	p := SetupParams{
		HostAddress:             n.HostAddress,
		HostUser:                n.HostUser,
		SystemdUnit:             n.HostSystemdUnitName,
		SystemctlPath:           n.HostSystemctlPath,
		NginxSbinPath:           n.SbinPath,
		HostConfigDir:           n.HostConfigDir,
		HostLogDir:              n.HostLogDir,
		ContainerKeyPath:        n.HostPrivateKeyPath,
		ContainerKnownHostsPath: n.HostKnownHostsPath,
	}
	return p.FillDefaults()
}
