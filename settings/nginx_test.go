package settings

import "testing"

func TestNginx_ControlMode(t *testing.T) {
	tests := []struct {
		name     string
		nginx    Nginx
		expected string
	}{
		{"default empty", Nginx{}, ControlModeLocal},
		{"container only", Nginx{ContainerName: "nginx-1"}, ControlModeExternalContainer},
		{"ssh only", Nginx{HostMode: HostModeSSH}, ControlModeHostViaSSH},
		{"ssh wins over container", Nginx{HostMode: HostModeSSH, ContainerName: "nginx-1"}, ControlModeHostViaSSH},
		{"unknown host mode falls back", Nginx{HostMode: "telnet"}, ControlModeLocal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nginx.ControlMode(); got != tt.expected {
				t.Errorf("ControlMode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNginx_RunningInAnotherContainer_UnchangedByHostMode(t *testing.T) {
	n := Nginx{HostMode: HostModeSSH}
	if n.RunningInAnotherContainer() {
		t.Errorf("RunningInAnotherContainer should remain false when only HostMode is set")
	}
}

func TestNginx_GetHostKnownHostsPath_Default(t *testing.T) {
	n := Nginx{}
	if got := n.GetHostKnownHostsPath(); got != "/etc/nginx-ui/known_hosts" {
		t.Errorf("GetHostKnownHostsPath() = %q, want %q", got, "/etc/nginx-ui/known_hosts")
	}
}

func TestNginx_GetHostKnownHostsPath_Configured(t *testing.T) {
	n := Nginx{HostKnownHostsPath: "/custom/known_hosts"}
	if got := n.GetHostKnownHostsPath(); got != "/custom/known_hosts" {
		t.Errorf("GetHostKnownHostsPath() = %q, want %q", got, "/custom/known_hosts")
	}
}

func TestNginx_GetHostSbinPath(t *testing.T) {
	tests := []struct {
		name  string
		nginx Nginx
		want  string
	}{
		{"systemd default", Nginx{HostMode: HostModeSSH}, DefaultHostSbinPathSystemd},
		{"launchd default", Nginx{HostMode: HostModeSSH, HostServiceManager: HostServiceManagerLaunchd}, DefaultHostSbinPathLaunchd},
		{"configured path wins", Nginx{HostMode: HostModeSSH, HostServiceManager: HostServiceManagerLaunchd, SbinPath: "/usr/local/bin/nginx"}, "/usr/local/bin/nginx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nginx.GetHostSbinPath(); got != tt.want {
				t.Errorf("GetHostSbinPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNginx_GetHostSystemdDefaults(t *testing.T) {
	tests := []struct {
		name          string
		nginx         Nginx
		wantSystemctl string
		wantUnit      string
	}{
		{"defaults", Nginx{}, DefaultHostSystemctlPath, DefaultHostSystemdUnitName},
		{"configured systemctl", Nginx{HostSystemctlPath: "/usr/bin/systemctl"}, "/usr/bin/systemctl", DefaultHostSystemdUnitName},
		{"configured unit", Nginx{HostSystemdUnitName: "openresty.service"}, DefaultHostSystemctlPath, "openresty.service"},
		{"both configured", Nginx{HostSystemctlPath: "/usr/bin/systemctl", HostSystemdUnitName: "openresty.service"}, "/usr/bin/systemctl", "openresty.service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nginx.GetHostSystemctlPath(); got != tt.wantSystemctl {
				t.Errorf("GetHostSystemctlPath() = %q, want %q", got, tt.wantSystemctl)
			}
			if got := tt.nginx.GetHostSystemdUnitName(); got != tt.wantUnit {
				t.Errorf("GetHostSystemdUnitName() = %q, want %q", got, tt.wantUnit)
			}
		})
	}
}

func TestNormalizeHostKeySource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		path   string
		want   string
	}{
		{"empty source with managed path is generated", "", DefaultHostPrivateKeyPath, HostKeySourceGenerated},
		{"empty source with custom path is existing", "", "/root/.ssh/id_ed25519", HostKeySourceExisting},
		{"empty source with empty path is existing", "", "", HostKeySourceExisting},
		{"generated passes through", HostKeySourceGenerated, "/root/.ssh/id_ed25519", HostKeySourceGenerated},
		{"existing passes through", HostKeySourceExisting, DefaultHostPrivateKeyPath, HostKeySourceExisting},
		{"provided passes through", HostKeySourceProvided, "", HostKeySourceProvided},
		// An unknown value must survive so the API validator can reject it.
		{"unknown passes through", "vault", DefaultHostPrivateKeyPath, "vault"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeHostKeySource(tt.source, tt.path); got != tt.want {
				t.Errorf("NormalizeHostKeySource(%q, %q) = %q, want %q", tt.source, tt.path, got, tt.want)
			}
		})
	}
}

// The persisted getter coerces unknown values and defaults an empty path,
// which is the one place its behaviour differs from the shared rule.
func TestNginx_GetHostKeySource(t *testing.T) {
	tests := []struct {
		name  string
		nginx Nginx
		want  string
	}{
		{"empty everything is generated", Nginx{}, HostKeySourceGenerated},
		{"empty source with custom path is existing", Nginx{HostPrivateKeyPath: "/root/.ssh/id_ed25519"}, HostKeySourceExisting},
		{"unknown source with managed path is generated", Nginx{HostKeySource: "vault"}, HostKeySourceGenerated},
		{"unknown source with custom path is existing", Nginx{HostKeySource: "vault", HostPrivateKeyPath: "/root/.ssh/id_ed25519"}, HostKeySourceExisting},
		{"provided wins over the path", Nginx{HostKeySource: HostKeySourceProvided, HostPrivateKeyPath: DefaultHostPrivateKeyPath}, HostKeySourceProvided},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nginx.GetHostKeySource(); got != tt.want {
				t.Errorf("GetHostKeySource() = %q, want %q", got, tt.want)
			}
		})
	}
}
