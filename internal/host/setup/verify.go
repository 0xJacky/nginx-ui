//go:build linux

package setup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"golang.org/x/sys/unix"
)

// StepOutcome is a single check result. Detail is the raw evidence;
// Remediation is a human-readable fix-it hint that the UI may render
// as a copy-pasteable shell command.
type StepOutcome struct {
	OK          bool   `json:"ok"`
	Level       string `json:"level,omitempty"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation,omitempty"`
}

// VerifyResult aggregates all step outcomes.
type VerifyResult struct {
	Steps map[string]StepOutcome `json:"steps"`
}

// VerifyOptions narrows the input to what verify actually needs.
type VerifyOptions struct {
	Client     *hostssh.Client
	Params     SetupParams
	SkipNginxT bool
}

// Verify runs the 10-step (including step 0) self-check pipeline.
func Verify(ctx context.Context, opts VerifyOptions) VerifyResult {
	r := VerifyResult{Steps: map[string]StepOutcome{}}
	p := opts.Params.FillDefaults()
	r.Steps["known_hosts_persistence"] = checkKnownHostsPersistence(p.ContainerKnownHostsPath)

	r.Steps["same_host"] = checkSameHost(ctx, opts.Client)

	connOut, err := opts.Client.Exec(ctx, "/bin/echo", "ok")
	r.Steps["ssh_connect"] = okOrFail(err, "echo ok over ssh",
		"Check SSH server is up, user exists, and key/password is correct.",
		connOut)
	if err != nil {
		return r
	}

	_, err = opts.Client.Exec(ctx, "/usr/bin/sudo", "-n", "/bin/true")
	r.Steps["sudo_available"] = okOrFail(err, "sudo -n true succeeded",
		"Re-check /etc/sudoers.d/nginx-ui content from Step 2b of the wizard.", "")

	listOut, listErr := opts.Client.Exec(ctx, "/usr/bin/sudo", "-n", "-l")
	if listErr != nil {
		r.Steps["sudoers_coverage"] = StepOutcome{OK: false, Detail: listErr.Error(),
			Remediation: "Run `sudo -l` on the host manually to inspect."}
	} else {
		required := []string{
			fmt.Sprintf("%s reload %s", p.SystemctlPath, p.SystemdUnit),
			fmt.Sprintf("%s restart %s", p.SystemctlPath, p.SystemdUnit),
			fmt.Sprintf("%s -t", p.NginxSbinPath),
			fmt.Sprintf("%s -T", p.NginxSbinPath),
		}
		missing := findMissingSudoEntries(listOut, required)
		if len(missing) == 0 {
			r.Steps["sudoers_coverage"] = StepOutcome{OK: true, Detail: "all required entries present"}
		} else {
			r.Steps["sudoers_coverage"] = StepOutcome{
				OK:          false,
				Detail:      "missing: " + strings.Join(missing, "; "),
				Remediation: "Append the missing entries to " + p.SudoersFilename + " (see Step 2b).",
			}
		}
	}

	isActiveOut, err := opts.Client.Exec(ctx, p.SystemctlPath, "is-active", p.SystemdUnit)
	r.Steps["systemctl_is_active"] = okOrFail(err,
		"is-active returned: "+strings.TrimSpace(isActiveOut),
		"Check that the systemd unit name matches your installation (e.g. nginx.service vs openresty.service).",
		isActiveOut)

	showOut, err := opts.Client.Exec(ctx, p.SystemctlPath, "show", p.SystemdUnit, "--property=ExecReload")
	if err == nil && !strings.Contains(showOut, "ExecReload={") && !strings.HasPrefix(strings.TrimSpace(showOut), "ExecReload=") {
		r.Steps["unit_has_execreload"] = StepOutcome{OK: false,
			Detail:      "ExecReload not declared in unit",
			Remediation: "Some packages omit ExecReload; reload via `systemctl restart` instead."}
	} else {
		r.Steps["unit_has_execreload"] = okOrFail(err, "ExecReload is declared", "Inspect unit file.", showOut)
	}

	if opts.SkipNginxT {
		r.Steps["nginx_test"] = StepOutcome{OK: true, Detail: "skipped by user request"}
	} else {
		ntOut, err := opts.Client.Exec(ctx, "/usr/bin/sudo", "-n", p.NginxSbinPath, "-t")
		r.Steps["nginx_test"] = okOrFail(err, strings.TrimSpace(ntOut),
			"Fix the nginx config error shown in detail.", ntOut)
	}

	r.Steps["config_dir_writable"] = checkDirAccess(p.ContainerConfigDir, true)
	r.Steps["log_dir_readable"] = checkLogReadable(p.ContainerLogDir + "/access.log")
	r.Steps["pid_file_present"] = checkPathExists("/var/run/nginx.pid")

	return r
}

func checkSameHost(ctx context.Context, c *hostssh.Client) StepOutcome {
	localID, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return StepOutcome{OK: false, Detail: "container has no /etc/machine-id"}
	}
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	remoteID, err := c.Exec(subCtx, "/bin/cat", "/etc/machine-id")
	if err != nil {
		return StepOutcome{OK: false, Detail: "could not read remote /etc/machine-id: " + err.Error(),
			Remediation: "If host is remote, see the cluster Node cross-host guide."}
	}
	if strings.TrimSpace(string(localID)) == strings.TrimSpace(remoteID) {
		return StepOutcome{OK: true, Detail: "machine-id matched"}
	}
	return StepOutcome{OK: false,
		Detail:      "remote host detected; bind-mount file I/O will not work for configs/logs",
		Remediation: "See cluster Node cross-host guide for proper deployment."}
}

func checkDirAccess(path string, writable bool) StepOutcome {
	info, err := os.Stat(path)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: fmt.Sprintf("Add a bind-mount: -v %s:%s", path, path)}
	}
	if !info.IsDir() {
		return StepOutcome{OK: false, Detail: path + " exists but is not a directory"}
	}
	mode := unix.W_OK | unix.R_OK
	if !writable {
		mode = unix.R_OK
	}
	if err := unix.Access(path, uint32(mode)); err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: "See ACL commands in Step 2b of the setup wizard."}
	}
	return StepOutcome{OK: true, Detail: path + " accessible"}
}

func checkLogReadable(path string) StepOutcome {
	f, err := os.Open(path)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: "Add user to 'adm' group on the host: usermod -aG adm <user>."}
	}
	defer f.Close()
	buf := make([]byte, 1)
	_, err = f.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return StepOutcome{OK: false, Detail: err.Error()}
	}
	return StepOutcome{OK: true, Detail: path + " readable"}
}

func checkPathExists(path string) StepOutcome {
	if _, err := os.Stat(path); err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: "Confirm `-v /var/run:/var/run:ro` in your compose file."}
	}
	return StepOutcome{OK: true, Detail: path + " present"}
}

func okOrFail(err error, okDetail, remediation, raw string) StepOutcome {
	if err == nil {
		return StepOutcome{OK: true, Detail: okDetail}
	}
	detail := err.Error()
	if raw != "" {
		detail = strings.TrimSpace(raw)
	}
	return StepOutcome{OK: false, Detail: detail, Remediation: remediation}
}

func findMissingSudoEntries(sudoListOutput string, required []string) []string {
	var missing []string
	for _, req := range required {
		if !strings.Contains(sudoListOutput, req) {
			missing = append(missing, req)
		}
	}
	return missing
}
