//go:build linux

package setup

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	Client     CommandRunner
	Params     SetupParams
	SkipNginxT bool
	// Groups limits the pipeline to the listed check groups. Empty runs all.
	Groups []CheckGroup
}

// Verify runs the self-check pipeline, optionally limited to a set of groups so
// a wizard step can validate what it just configured.
func Verify(ctx context.Context, opts VerifyOptions) VerifyResult {
	r := VerifyResult{Steps: map[string]StepOutcome{}}
	p := opts.Params.FillDefaults()

	// The same values the wizard refuses to paste into a root shell are the
	// ones the checks below stat and open, so they are validated here too.
	if err := p.ValidateSnippetValues(); err != nil {
		r.Steps["parameters"] = StepOutcome{OK: false, Detail: err.Error(),
			Remediation: "Correct the highlighted path or identifier and run verification again."}
		return r
	}

	wants := newCheckGroupFilter(opts.Groups)

	if wants(CheckGroupConnection) {
		r.Steps["known_hosts_persistence"] = checkKnownHostsPersistence(p.ContainerKnownHostsPath)
	}

	// Every remaining check needs a working session, so this one always runs.
	connOut, err := opts.Client.Exec(ctx, "/bin/echo", "ok")
	r.Steps["ssh_connect"] = okOrFail(err, "echo ok over ssh",
		"Check SSH server is up, user exists, and key/password is correct.",
		connOut)
	if err != nil {
		return r
	}

	if wants(CheckGroupConnection) {
		r.Steps["same_host"] = checkSameHost(ctx, opts.Client, p)
	}

	if wants(CheckGroupPlatform) {
		r.Steps["host_platform"] = checkHostPlatform(ctx, opts.Client, p)
		if p.IsLaunchd() {
			verifyLaunchd(ctx, opts.Client, p, r.Steps)
		} else {
			verifySystemdService(ctx, opts.Client, p, r.Steps)
		}
		r.Steps["config_dir_writable"] = checkDirAccess(p.ContainerConfigDir, true)
		r.Steps["config_dir_shared"] = checkSharedPath(ctx, opts.Client, p, p.ContainerConfigDir, p.HostConfigDir)
		r.Steps["log_dir_readable"] = checkLogReadable(p.ContainerLogDir + "/access.log")
		r.Steps["pid_file_present"] = checkPathExists(p.PIDPath)
	}

	// A target that needs no sudoers entry reports no privilege steps at all,
	// so the UI does not show checks the operator cannot act on.
	if wants(CheckGroupPrivileges) && sudoersRequired(ctx, opts.Client, p) {
		verifySudoers(ctx, opts.Client, p, r.Steps)
	}

	if wants(CheckGroupNginx) {
		if opts.SkipNginxT {
			// A check that did not run must not read as a pass.
			r.Steps["nginx_test"] = StepOutcome{
				OK:     false,
				Level:  "warning",
				Detail: "skipped by user request; the configuration was not validated",
			}
		} else {
			ntOut, ntErr := opts.Client.Exec(ctx, p.NginxSbinPath, "-t")
			r.Steps["nginx_test"] = okOrFail(ntErr, strings.TrimSpace(ntOut),
				"Fix the nginx config error shown in detail.", ntOut)
		}
	}

	return r
}

// sudoersRequired reports whether the SSH user needs a sudoers entry. A launchd
// service runs in the user's own domain and root already holds every privilege.
// An unreadable remote uid falls back to true so the checks still run.
func sudoersRequired(ctx context.Context, client CommandRunner, p SetupParams) bool {
	if !p.NeedsSudoers() {
		return false
	}
	uidOut, err := client.Exec(ctx, "/usr/bin/id", "-u")
	if err != nil {
		return true
	}
	return strings.TrimSpace(uidOut) != "0"
}

// verifySudoers covers the privileges group.
func verifySudoers(ctx context.Context, client CommandRunner, p SetupParams, steps map[string]StepOutcome) {
	_, err := client.Exec(ctx, "/usr/bin/sudo", "-n", "/bin/true")
	steps["sudo_available"] = okOrFail(err, "sudo -n true succeeded",
		"Re-check the sudoers rules shown in the wizard Install step.", "")

	listOut, listErr := client.Exec(ctx, "/usr/bin/sudo", "-n", "-l")
	if listErr != nil {
		steps["sudoers_coverage"] = StepOutcome{OK: false, Detail: listErr.Error(),
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
			steps["sudoers_coverage"] = StepOutcome{OK: true, Detail: "all required entries present"}
		} else {
			steps["sudoers_coverage"] = StepOutcome{OK: false,
				Detail: "missing: " + strings.Join(missing, "; "), Remediation: "Append the missing entries to " + p.SudoersFilename + " (see the wizard Install step)."}
		}
	}
}

// verifySystemdService covers the systemd part of the platform group.
func verifySystemdService(ctx context.Context, client CommandRunner, p SetupParams, steps map[string]StepOutcome) {
	isActiveOut, err := client.Exec(ctx, p.SystemctlPath, "is-active", p.SystemdUnit)
	steps["systemctl_is_active"] = okOrFail(err, "is-active returned: "+strings.TrimSpace(isActiveOut),
		"Check that the systemd unit name matches your installation (e.g. nginx.service vs openresty.service).", isActiveOut)

	showOut, err := client.Exec(ctx, p.SystemctlPath, "show", p.SystemdUnit, "--property=ExecReload")
	if err != nil {
		steps["unit_has_execreload"] = okOrFail(err, "", "Inspect unit file.", showOut)
		return
	}
	// systemctl always prints the property name, so an undeclared ExecReload
	// shows up as an empty value rather than a missing line.
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(showOut), "ExecReload="))
	if value == "" {
		steps["unit_has_execreload"] = StepOutcome{OK: false, Detail: "ExecReload is not declared in the unit",
			Remediation: "Some packages omit ExecReload; reload via `systemctl restart` instead."}
		return
	}
	steps["unit_has_execreload"] = StepOutcome{OK: true, Detail: "ExecReload is declared"}
}

func verifyLaunchd(ctx context.Context, client CommandRunner, p SetupParams, steps map[string]StepOutcome) {
	uidOut, err := client.Exec(ctx, "/usr/bin/id", "-u")
	if err != nil {
		steps["launchctl_service_loaded"] = StepOutcome{OK: false, Detail: err.Error(), Remediation: "Confirm the SSH user owns the Homebrew service."}
		return
	}
	uid := strings.TrimSpace(uidOut)
	parsedUID, err := strconv.ParseUint(uid, 10, 32)
	if err != nil || parsedUID == 0 {
		steps["launchctl_service_loaded"] = StepOutcome{OK: false, Detail: "invalid remote user id: " + uid}
		return
	}
	target := "gui/" + uid + "/" + p.LaunchdService
	out, err := client.Exec(ctx, p.LaunchctlPath, "print", target)
	steps["launchctl_service_loaded"] = okOrFail(err, target+" is loaded",
		"Run `brew services start nginx` as the configured SSH user.", out)
}

func checkHostPlatform(ctx context.Context, c CommandRunner, p SetupParams) StepOutcome {
	out, err := c.Exec(ctx, "/usr/bin/uname", "-s")
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(), Remediation: "Confirm /usr/bin/uname exists on the host."}
	}
	got := strings.TrimSpace(out)
	want := "Linux"
	if p.IsLaunchd() {
		want = "Darwin"
	}
	if got != want {
		return StepOutcome{OK: false, Detail: "expected " + want + ", got " + got,
			Remediation: "Select the service manager matching the SSH host."}
	}
	return StepOutcome{OK: true, Detail: got + " host matches " + p.ServiceManager}
}

// checkTargetIsDefaultGateway looks for evidence that the configured gateway
// alias really points at the container host. UseHostGateway arrives from the
// request, so it cannot stand in for the check on its own.
func checkTargetIsDefaultGateway(hostAddress string) StepOutcome {
	host := hostAddress
	if parsed, _, err := net.SplitHostPort(hostAddress); err == nil {
		host = parsed
	}
	gateway := defaultGatewayIP()
	if gateway == "" {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "could not read the container default gateway, so the target could not be confirmed as the container host",
			Remediation: "Confirm the address reaches the machine that runs nginx."}
	}
	addresses, err := net.LookupHost(host)
	if err != nil {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "could not resolve " + host + ": " + err.Error(),
			Remediation: "Confirm the address reaches the machine that runs nginx."}
	}
	for _, address := range addresses {
		if address == gateway {
			return StepOutcome{OK: true, Detail: host + " resolves to the container default gateway " + gateway}
		}
	}
	return StepOutcome{OK: false, Level: "warning",
		Detail: host + " resolves to " + strings.Join(addresses, ", ") +
			", which is not the container default gateway " + gateway,
		Remediation: "On Docker Desktop this is expected. Elsewhere, confirm the address reaches the machine that runs nginx."}
}

func checkSameHost(ctx context.Context, c CommandRunner, p SetupParams) StepOutcome {
	if p.UseHostGateway {
		return checkTargetIsDefaultGateway(p.HostAddress)
	}
	if p.IsLaunchd() {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "macOS has no machine-id; same-host access is validated by the config, log, and PID bind mounts",
			Remediation: "Use host.docker.internal when nginx runs on the Docker host."}
	}
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
			Remediation: "See the file permission commands in the wizard Install step."}
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

// checkSharedPath proves the container path and the host path are the same
// directory. Both sides existing is not evidence on its own: the nginx-ui image
// ships its own /etc/nginx, so a missing bind mount still passes a local stat
// while every edit lands in the container instead of on the host.
func checkSharedPath(ctx context.Context, c CommandRunner, p SetupParams, containerPath, hostPath string) StepOutcome {
	if containerPath == "" || hostPath == "" {
		return StepOutcome{OK: false, Level: "warning",
			Detail: "no host directory is configured, so the bind mount could not be confirmed"}
	}

	info, err := os.Stat(containerPath)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: fmt.Sprintf("Bind-mount the host directory at %s.", containerPath)}
	}
	local, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return StepOutcome{OK: false, Level: "warning", Detail: "could not read the local inode"}
	}

	// A bind mount exposes the host inode unchanged, so comparing inodes needs
	// no write and leaves nothing behind.
	statArgs := []string{"-c", "%i", hostPath}
	if p.IsLaunchd() {
		statArgs = []string{"-f", "%i", hostPath}
	}
	subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := c.Exec(subCtx, "/usr/bin/stat", statArgs...)
	if err != nil {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "could not read the host inode for " + hostPath + ": " + err.Error(),
			Remediation: "The bind mount could not be confirmed; verify it manually."}
	}
	remote, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return StepOutcome{OK: false, Level: "warning",
			Detail: "unexpected stat output for " + hostPath + ": " + strings.TrimSpace(out)}
	}
	if remote != local.Ino {
		return StepOutcome{OK: false,
			Detail: fmt.Sprintf("%s and host %s are different directories (inode %d vs %d)",
				containerPath, hostPath, local.Ino, remote),
			Remediation: fmt.Sprintf("Add a bind-mount: -v %s:%s", hostPath, containerPath)}
	}
	return StepOutcome{OK: true,
		Detail: fmt.Sprintf("%s is the host directory %s (inode %d)", containerPath, hostPath, remote)}
}

func checkPathExists(path string) StepOutcome {
	if _, err := os.Stat(path); err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: fmt.Sprintf("Bind-mount the host PID directory containing %s at the same container path.", path)}
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
