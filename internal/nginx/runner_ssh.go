package nginx

import (
	"context"
	"sync"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// sshOnce ensures we share a single long-lived SSH client across all Exec calls.
//
// LIMITATION: if settings.NginxSettings.Host* fields change at runtime (e.g.
// the user saves new SSH config via the Web UI), the cached client is NOT
// rebuilt with the new settings. Settings handler must call ResetSSHClient()
// after writes that affect host SSH config. See spec §6.3.
var (
	sshOnce   sync.Once
	sshShared *hostssh.Client
)

func newSSHRunner() Runner {
	sshOnce.Do(func() {
		opts := buildSSHOptions()
		sshShared = hostssh.NewClient(opts)
	})
	return &sshRunner{client: sshShared}
}

// ResetSSHClient invalidates the cached SSH client so the next nginx command
// re-dials with the current settings. Safe to call concurrently with Exec.
func ResetSSHClient() {
	if sshShared != nil {
		_ = sshShared.Close()
	}
	sshOnce = sync.Once{}
	sshShared = nil
}

func buildSSHOptions() hostssh.ClientOptions {
	n := settings.NginxSettings

	kh, err := hostssh.NewKnownHosts(n.GetHostKnownHostsPath())
	if err != nil {
		logger.Error("Failed to initialize SSH known_hosts allow-list", err)
	}

	password := ""
	if n.HostAuthMethod == "password" {
		logger.Warn("SSH password auth is configured but not yet supported (pending crypto package refactor); SSH connections will fail until you switch to key auth")
	}
	_ = n.HostPasswordRef // suppress unused-field lint until decryption lands

	sudo := n.HostSudoPrefix
	if sudo == "" {
		sudo = "sudo -n"
	}
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}

	return hostssh.ClientOptions{
		Address:        n.HostAddress,
		User:           n.HostUser,
		AuthMethod:     n.HostAuthMethod,
		PrivateKeyPath: n.HostPrivateKeyPath,
		Password:       password,
		KnownHosts:     kh,
		Config: hostssh.Config{
			SudoPrefix:    sudo,
			SystemctlPath: systemctl,
			NginxSbinPath: n.SbinPath,
		},
	}
}

type sshRunner struct {
	client *hostssh.Client
}

func (s *sshRunner) Exec(ctx context.Context, name string, args ...string) (string, error) {
	return s.client.Exec(ctx, name, args...)
}

func (s *sshRunner) Stat(path string) bool {
	return s.client.Stat(path)
}
