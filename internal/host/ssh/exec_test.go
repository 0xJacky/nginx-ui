package ssh

import "testing"

func TestBuildCommand_NoSudoForBareNginxV(t *testing.T) {
	cfg := Config{SudoPrefix: "sudo -n", SystemctlPath: "/bin/systemctl", NginxSbinPath: "/usr/sbin/nginx"}
	got := buildCommand(cfg, "/usr/sbin/nginx", []string{"-V"})
	want := "/usr/sbin/nginx -V"
	if got != want {
		t.Errorf("buildCommand(nginx -V) = %q, want %q", got, want)
	}
}

func TestBuildCommand_SudoForReload(t *testing.T) {
	cfg := Config{SudoPrefix: "sudo -n", SystemctlPath: "/bin/systemctl", NginxSbinPath: "/usr/sbin/nginx"}
	got := buildCommand(cfg, "/bin/systemctl", []string{"reload", "nginx.service"})
	want := "sudo -n /bin/systemctl reload nginx.service"
	if got != want {
		t.Errorf("buildCommand(systemctl reload) = %q, want %q", got, want)
	}
}

func TestBuildCommand_SudoForNginxT(t *testing.T) {
	cfg := Config{SudoPrefix: "sudo -n", SystemctlPath: "/bin/systemctl", NginxSbinPath: "/usr/sbin/nginx"}
	got := buildCommand(cfg, "/usr/sbin/nginx", []string{"-t"})
	want := "sudo -n /usr/sbin/nginx -t"
	if got != want {
		t.Errorf("buildCommand(nginx -t) = %q, want %q", got, want)
	}
}

func TestBuildCommand_NoSudoForIsActive(t *testing.T) {
	cfg := Config{SudoPrefix: "sudo -n", SystemctlPath: "/bin/systemctl", NginxSbinPath: "/usr/sbin/nginx"}
	got := buildCommand(cfg, "/bin/systemctl", []string{"is-active", "nginx.service"})
	want := "/bin/systemctl is-active nginx.service"
	if got != want {
		t.Errorf("buildCommand(systemctl is-active) = %q, want %q", got, want)
	}
}

func TestBuildCommand_ShellEscape(t *testing.T) {
	cfg := Config{SystemctlPath: "/bin/systemctl"}
	got := buildCommand(cfg, "echo", []string{"hello world", "with'quote"})
	want := `echo 'hello world' 'with'\''quote'`
	if got != want {
		t.Errorf("buildCommand(escape) = %q, want %q", got, want)
	}
}
