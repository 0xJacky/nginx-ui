package setup

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func validSSHControlSettings() ControlSettings {
	return ControlSettings{
		Mode:               settings.ControlModeHostViaSSH,
		HostAddress:        "host.docker.internal:22",
		HostUser:           "nginxui",
		HostAccessMode:     settings.HostAccessModeSFTP,
		HostAuthMethod:     "key",
		HostKeySource:      settings.HostKeySourceGenerated,
		HostPrivateKeyPath: "/etc/nginx-ui/host_key",
		HostKnownHostsPath: "/etc/nginx-ui/known_hosts",
		HostServiceManager: settings.HostServiceManagerSystemd,
	}
}

func TestValidateControlSettings(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(s *ControlSettings)
		wantErr error
	}{
		{
			name:   "local ignores other fields",
			mutate: func(s *ControlSettings) { s.Mode = settings.ControlModeLocal; s.HostUser = "not valid!" },
		},
		{
			name:   "external container",
			mutate: func(s *ControlSettings) { s.Mode = settings.ControlModeExternalContainer; s.ContainerName = "nginx-1" },
		},
		{
			name:    "external container requires name",
			mutate:  func(s *ControlSettings) { s.Mode = settings.ControlModeExternalContainer },
			wantErr: ErrContainerNameRequired,
		},
		{
			name: "external container rejects invalid name",
			mutate: func(s *ControlSettings) {
				s.Mode = settings.ControlModeExternalContainer
				s.ContainerName = "nginx container"
			},
			wantErr: ErrInvalidContainerName,
		},
		{
			name: "external container rejects overlong name",
			mutate: func(s *ControlSettings) {
				s.Mode = settings.ControlModeExternalContainer
				s.ContainerName = strings.Repeat("a", maxContainerNameLength+1)
			},
			wantErr: ErrInvalidContainerName,
		},
		{
			name:   "ssh",
			mutate: func(*ControlSettings) {},
		},
		{
			name: "ssh with launchd and provided key",
			mutate: func(s *ControlSettings) {
				s.HostServiceManager = settings.HostServiceManagerLaunchd
				s.HostKeySource = settings.HostKeySourceProvided
			},
		},
		{
			name:    "ssh requires address",
			mutate:  func(s *ControlSettings) { s.HostAddress = "" },
			wantErr: ErrSSHConnectionRequired,
		},
		{
			name:    "ssh requires user",
			mutate:  func(s *ControlSettings) { s.HostUser = "" },
			wantErr: ErrSSHConnectionRequired,
		},
		{
			name:    "ssh rejects unsafe user",
			mutate:  func(s *ControlSettings) { s.HostUser = "Nginx UI" },
			wantErr: ErrInvalidHostUser,
		},
		{
			name:    "ssh rejects unknown access mode",
			mutate:  func(s *ControlSettings) { s.HostAccessMode = "nfs" },
			wantErr: ErrInvalidAccessMode,
		},
		{
			name:    "ssh requires key auth",
			mutate:  func(s *ControlSettings) { s.HostAuthMethod = "password" },
			wantErr: ErrSSHKeyAuthRequired,
		},
		{
			name:    "ssh rejects unknown key source",
			mutate:  func(s *ControlSettings) { s.HostKeySource = "agent" },
			wantErr: ErrInvalidKeySource,
		},
		{
			name:    "ssh requires private key path",
			mutate:  func(s *ControlSettings) { s.HostPrivateKeyPath = "" },
			wantErr: ErrSSHKeyPathsRequired,
		},
		{
			name:    "ssh requires known hosts path",
			mutate:  func(s *ControlSettings) { s.HostKnownHostsPath = "" },
			wantErr: ErrSSHKeyPathsRequired,
		},
		{
			name:    "ssh rejects unsupported service manager",
			mutate:  func(s *ControlSettings) { s.HostServiceManager = "openrc" },
			wantErr: ErrUnsupportedServiceManager,
		},
		{
			name:    "unknown mode",
			mutate:  func(s *ControlSettings) { s.Mode = "remote" },
			wantErr: ErrInvalidControlMode,
		},
		{
			name:    "empty mode",
			mutate:  func(s *ControlSettings) { s.Mode = "" },
			wantErr: ErrInvalidControlMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSSHControlSettings()
			tt.mutate(&s)
			err := ValidateControlSettings(s)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
