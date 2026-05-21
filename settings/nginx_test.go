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
