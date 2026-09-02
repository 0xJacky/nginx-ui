package ssh

import "strings"

// Config bundles per-invocation settings for command building.
type Config struct {
	SudoPrefix    string // e.g. "sudo -n"; empty disables sudo wrapping
	SystemctlPath string // resolved at verify time, e.g. "/bin/systemctl"
	NginxSbinPath string // resolved at verify time, e.g. "/usr/sbin/nginx"
}

// needsSudo decides whether a given remote invocation requires the sudo prefix.
// Rules mirror the sudoers whitelist:
//   - systemctl reload|restart <unit>  → sudo
//   - <nginx> -t | <nginx> -T          → sudo
//   - systemctl is-active|status|show  → no sudo (query commands)
//   - <nginx> -V                       → no sudo (prints compile info)
func needsSudo(cfg Config, name string, args []string) bool {
	if name == cfg.SystemctlPath && len(args) >= 1 {
		switch args[0] {
		case "reload", "restart":
			return true
		}
		return false
	}
	if name == cfg.NginxSbinPath && len(args) >= 1 {
		switch args[0] {
		case "-t", "-T":
			return true
		}
		return false
	}
	return false
}

// ShellQuote quotes s as a single POSIX shell word. Tokens made only of
// characters no shell interprets are returned bare so commands stay readable;
// anything else is single-quoted, with an inner single quote written as a
// closing quote, a backslash-escaped quote and a reopening quote. It is the
// one quoting rule for commands executed over SSH and for the copy-and-paste
// snippets the setup wizard renders, so both agree on every value.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !needsQuoting(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func needsQuoting(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '/', '.', '-', '_', '=', ':', ',', '@', '+':
			continue
		}
		return true
	}
	return false
}

// buildCommand assembles the final shell string that gets passed to
// gossh.Session.Run.
func buildCommand(cfg Config, name string, args []string) string {
	var b strings.Builder
	if needsSudo(cfg, name, args) && cfg.SudoPrefix != "" {
		// SudoPrefix is split on whitespace and each token is shell-quoted
		// to prevent injection via misconfigured settings.
		for i, tok := range strings.Fields(cfg.SudoPrefix) {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(ShellQuote(tok))
		}
		b.WriteByte(' ')
	}
	b.WriteString(ShellQuote(name))
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(ShellQuote(a))
	}
	return b.String()
}
