package settings

import "testing"

func TestNginx_ControlMode(t *testing.T) {
	tests := []struct {
		name     string
		nginx    Nginx
		expected string
	}{
		{"default empty", Nginx{}, "local"},
		{"container only", Nginx{ContainerName: "nginx-1"}, "external_container"},
		{"ssh only", Nginx{HostMode: "ssh"}, "host_via_ssh"},
		{"ssh wins over container", Nginx{HostMode: "ssh", ContainerName: "nginx-1"}, "host_via_ssh"},
		{"unknown host mode falls back", Nginx{HostMode: "telnet"}, "local"},
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
	n := Nginx{HostMode: "ssh"}
	if n.RunningInAnotherContainer() {
		t.Errorf("RunningInAnotherContainer should remain false when only HostMode is set")
	}
}
