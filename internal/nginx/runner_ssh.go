package nginx

import (
	"context"
	"sync"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// A single long-lived SSH client is shared across all Exec calls.
//
// LIMITATION: if settings.NginxSettings.Host* fields change at runtime (e.g.
// the user saves new SSH config via the Web UI), the cached client is NOT
// rebuilt with the new settings. Settings handler must call ResetSSHClient()
// after writes that affect host SSH config. See spec §6.3.
var (
	sshMutex  sync.Mutex
	sshShared *hostssh.Client
)

// sharedSSHClient returns the cached client, building it on first use. The
// mutex covers both the read and the build so a concurrent reset cannot hand
// out a nil client.
func sharedSSHClient() *hostssh.Client {
	sshMutex.Lock()
	defer sshMutex.Unlock()
	if sshShared == nil {
		sshShared = hostssh.NewClient(buildSSHOptions())
	}
	return sshShared
}

func newSSHRunner() Runner {
	return &sshRunner{client: sharedSSHClient()}
}

// ResetHostNginxState invalidates everything derived from the current nginx
// control target: the SSH client and the paths resolved from nginx -V/-T.
// Call it after any settings write that can change the target.
func ResetHostNginxState() {
	ResetSSHClient()
	resetPathCaches()
}

// ResetSSHClient invalidates the cached SSH client so the next nginx command
// re-dials with the current settings. Safe to call concurrently with Exec: a
// runner already holds its own reference, so it finishes against the old
// client instead of observing a nil one.
func ResetSSHClient() {
	sshMutex.Lock()
	previous := sshShared
	sshShared = nil
	sshMutex.Unlock()

	if previous != nil {
		_ = previous.Close()
	}
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

	sudo := n.GetHostSudoPrefix()
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
			// The resolved path must match what -t/-T is invoked with, or the
			// sudo whitelist check in needsSudo never fires.
			NginxSbinPath: n.GetHostSbinPath(),
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
