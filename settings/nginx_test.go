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
