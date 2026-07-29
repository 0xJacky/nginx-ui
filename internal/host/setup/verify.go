//go:build linux

package setup

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"golang.org/x/sys/unix"
)

// StepOutcome is a single check result. Detail is raw evidence, while
// Remediation is a language-neutral message contract rendered by the UI.
type StepOutcome struct {
	OK          bool             `json:"ok"`
	Level       string           `json:"level,omitempty"`
	Detail      string           `json:"detail"`
	Remediation *StepRemediation `json:"remediation,omitempty"`
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
			Remediation: newStepRemediation(remediationCorrectParameters)}
		return r
	}

	wants := newCheckGroupFilter(opts.Groups)

	if wants(CheckGroupConnection) {
		r.Steps["known_hosts_persistence"] = checkKnownHostsPersistence(p.ContainerKnownHostsPath)
	}

	// Every remaining check needs a working session, so this one always runs.
	connOut, err := opts.Client.Exec(ctx, "/bin/echo", "ok")
	r.Steps["ssh_connect"] = okOrFail(err, "echo ok over ssh",
		remediationCheckSSHConnection,
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
		if p.AccessMode == settings.HostAccessModeSFTP {
			r.Steps["config_dir_writable"] = checkRemoteDirectory(ctx, opts.Client, p.HostConfigDir, true)
			r.Steps["log_dir_readable"] = checkRemoteDirectory(ctx, opts.Client, p.HostLogDir, false)
			r.Steps["pid_file_present"] = checkRemotePath(ctx, opts.Client, p.PIDPath)
		} else {
			r.Steps["config_dir_writable"] = checkDirAccess(p.ContainerConfigDir, true)
			r.Steps["config_dir_shared"] = checkSharedPath(ctx, opts.Client, p, p.ContainerConfigDir, p.HostConfigDir)
			r.Steps["log_dir_readable"] = checkLogReadable(p.ContainerLogDir + "/access.log")
			r.Steps["pid_file_present"] = checkPathExists(p.PIDPath)
		}
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
				remediationFixNginxConfig, ntOut)
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
		remediationReviewSudoersRules, "")

	listOut, listErr := client.Exec(ctx, "/usr/bin/sudo", "-n", "-l")
	if listErr != nil {
		steps["sudoers_coverage"] = StepOutcome{OK: false, Detail: listErr.Error(),
			Remediation: newStepRemediation(remediationInspectSudoPermissions)}
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
				Detail: "missing: " + strings.Join(missing, "; "), Remediation: newStepRemediation(remediationAddMissingSudoersEntries, map[string]string{
					"path": p.SudoersFilename,
				})}
		}
	}
}

// verifySystemdService covers the systemd part of the platform group.
func verifySystemdService(ctx context.Context, client CommandRunner, p SetupParams, steps map[string]StepOutcome) {
	isActiveOut, err := client.Exec(ctx, p.SystemctlPath, "is-active", p.SystemdUnit)
	steps["systemctl_is_active"] = okOrFail(err, "is-active returned: "+strings.TrimSpace(isActiveOut),
		remediationCheckSystemdUnit, isActiveOut)

	showOut, err := client.Exec(ctx, p.SystemctlPath, "show", p.SystemdUnit, "--property=ExecReload")
	if err != nil {
		steps["unit_has_execreload"] = okOrFail(err, "", remediationInspectSystemdUnit, showOut)
		return
	}
	// systemctl always prints the property name, so an undeclared ExecReload
	// shows up as an empty value rather than a missing line.
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(showOut), "ExecReload="))
	if value == "" {
		steps["unit_has_execreload"] = StepOutcome{OK: false, Detail: "ExecReload is not declared in the unit",
			Remediation: newStepRemediation(remediationRestartWithoutExecReload)}
		return
	}
	steps["unit_has_execreload"] = StepOutcome{OK: true, Detail: "ExecReload is declared"}
}

func verifyLaunchd(ctx context.Context, client CommandRunner, p SetupParams, steps map[string]StepOutcome) {
	uidOut, err := client.Exec(ctx, "/usr/bin/id", "-u")
	if err != nil {
		steps["launchctl_service_loaded"] = StepOutcome{OK: false, Detail: err.Error(), Remediation: newStepRemediation(remediationConfirmHomebrewServiceOwner)}
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
		remediationStartHomebrewNginx, out)
}

func checkHostPlatform(ctx context.Context, c CommandRunner, p SetupParams) StepOutcome {
	out, err := c.Exec(ctx, "/usr/bin/uname", "-s")
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(), Remediation: newStepRemediation(remediationConfirmUname)}
	}
	got := strings.TrimSpace(out)
	want := "Linux"
	if p.IsLaunchd() {
		want = "Darwin"
	}
	if got != want {
		return StepOutcome{OK: false, Detail: "expected " + want + ", got " + got,
			Remediation: newStepRemediation(remediationSelectServiceManager)}
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
			Remediation: newStepRemediation(remediationCheckHostAddress)}
	}
	addresses, err := net.LookupHost(host)
	if err != nil {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "could not resolve " + host + ": " + err.Error(),
			Remediation: newStepRemediation(remediationCheckHostAddress)}
	}
	for _, address := range addresses {
		if address == gateway {
			return StepOutcome{OK: true, Detail: host + " resolves to the container default gateway " + gateway}
		}
	}
	return StepOutcome{OK: false, Level: "warning",
		Detail: host + " resolves to " + strings.Join(addresses, ", ") +
			", which is not the container default gateway " + gateway,
		Remediation: newStepRemediation(remediationCheckHostAddressOutsideDocker)}
}

func checkMacOSHostGateway(hostAddress string, lookupHost func(string) ([]string, error)) StepOutcome {
	host := hostAddress
	if parsed, _, err := net.SplitHostPort(hostAddress); err == nil {
		host = parsed
	}
	if host != "host.docker.internal" {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      host + " is not the standard macOS container host alias",
			Remediation: newStepRemediation(remediationUseMacOSHostAlias)}
	}
	addresses, err := lookupHost(host)
	if err != nil || len(addresses) == 0 {
		detail := "no address was returned"
		if err != nil {
			detail = err.Error()
		}
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "could not resolve " + host + ": " + detail,
			Remediation: newStepRemediation(remediationConfirmMacOSHostAlias)}
	}
	return StepOutcome{OK: true,
		Detail: host + " resolves to the macOS host gateway " + strings.Join(addresses, ", ")}
}

func checkSameHost(ctx context.Context, c CommandRunner, p SetupParams) StepOutcome {
	if p.AccessMode == settings.HostAccessModeSFTP {
		return StepOutcome{OK: true, Detail: "SFTP access supports a remote nginx host"}
	}
	if p.IsLaunchd() && p.UseHostGateway {
		return checkMacOSHostGateway(p.HostAddress, net.LookupHost)
	}
	if p.UseHostGateway {
		return checkTargetIsDefaultGateway(p.HostAddress)
	}
	if p.IsLaunchd() {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "macOS has no machine-id; same-host access is validated by the config, log, and PID bind mounts",
			Remediation: newStepRemediation(remediationUseDockerHostAlias)}
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
			Remediation: newStepRemediation(remediationReviewCrossHostGuide)}
	}
	if strings.TrimSpace(string(localID)) == strings.TrimSpace(remoteID) {
		return StepOutcome{OK: true, Detail: "machine-id matched"}
	}
	return StepOutcome{OK: false,
		Detail:      "remote host detected; bind-mount file I/O will not work for configs/logs",
		Remediation: newStepRemediation(remediationUseClusterNode)}
}

func checkRemoteDirectory(ctx context.Context, c CommandRunner, path string, writable bool) StepOutcome {
	arguments := []string{"-d", path, "-a", "-r", path, "-a", "-x", path}
	if writable {
		arguments = append(arguments, "-a", "-w", path)
	}
	_, err := c.Exec(ctx, "/bin/test", arguments...)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: newStepRemediation(remediationReviewInstallPermissions)}
	}
	return StepOutcome{OK: true, Detail: path + " accessible over SFTP"}
}

func checkRemotePath(ctx context.Context, c CommandRunner, path string) StepOutcome {
	_, err := c.Exec(ctx, "/bin/test", "-e", path)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error()}
	}
	return StepOutcome{OK: true, Detail: path + " present on the SSH host"}
}

func checkDirAccess(path string, writable bool) StepOutcome {
	info, err := os.Stat(path)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: newStepRemediation(remediationAddBindMount, map[string]string{"source": path, "target": path})}
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
			Remediation: newStepRemediation(remediationReviewInstallPermissions)}
	}
	return StepOutcome{OK: true, Detail: path + " accessible"}
}

func checkLogReadable(path string) StepOutcome {
	f, err := os.Open(path)
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: newStepRemediation(remediationMountNginxLogs)}
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
			Remediation: newStepRemediation(remediationAddBindMount, map[string]string{"source": hostPath, "target": containerPath})}
	}
	if p.IsLaunchd() {
		return checkVirtualizedSharedPath(ctx, c, containerPath, hostPath)
	}
	local, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return StepOutcome{OK: false, Level: "warning", Detail: "could not read the local inode"}
	}

	// A bind mount exposes the host inode unchanged, so comparing inodes needs
	// no write and leaves nothing behind.
	subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := c.Exec(subCtx, "/usr/bin/stat", "-c", "%i", hostPath)
	if err != nil {
		return StepOutcome{OK: false, Level: "warning",
			Detail:      "could not read the host inode for " + hostPath + ": " + err.Error(),
			Remediation: newStepRemediation(remediationVerifyBindMount)}
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
			Remediation: newStepRemediation(remediationAddBindMount, map[string]string{"source": hostPath, "target": containerPath})}
	}
	return StepOutcome{OK: true,
		Detail: fmt.Sprintf("%s is the host directory %s (inode %d)", containerPath, hostPath, remote)}
}

// A macOS bind mount crosses a VM filesystem boundary, so its Linux-side inode
// is not expected to match the inode reported by the host. The mount table still
// records the host root selected by Docker Desktop, OrbStack, or another VM.
func checkVirtualizedSharedPath(ctx context.Context, c CommandRunner, containerPath, hostPath string) StepOutcome {
	subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := c.Exec(subCtx, "/usr/bin/stat", "-f", "%HT", hostPath); err != nil {
		return StepOutcome{OK: false, Detail: "could not stat the host directory " + hostPath + ": " + err.Error(),
			Remediation: newStepRemediation(remediationConfirmHostNginxDirectory)}
	}

	mountInfo, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return StepOutcome{OK: false, Level: "warning", Detail: "could not read the container mount table: " + err.Error(),
			Remediation: newStepRemediation(remediationVerifyBindMount)}
	}
	mountedHostPath, ok := mountedHostPathFor(mountInfo, containerPath)
	if !ok {
		return StepOutcome{OK: false,
			Detail:      containerPath + " is not backed by a separate container mount",
			Remediation: newStepRemediation(remediationAddBindMount, map[string]string{"source": hostPath, "target": containerPath})}
	}
	if !virtualizedMountRootMatches(mountedHostPath, hostPath) {
		return StepOutcome{OK: false,
			Detail: fmt.Sprintf("%s is mounted from %s instead of host %s",
				containerPath, mountedHostPath, hostPath),
			Remediation: newStepRemediation(remediationReplaceBindMount, map[string]string{"source": hostPath, "target": containerPath})}
	}
	return StepOutcome{OK: true,
		Detail: fmt.Sprintf("%s is mounted from host %s", containerPath, hostPath)}
}

// mountedHostPathFor resolves target through the deepest mount that contains
// it. The root field in mountinfo identifies the host-side directory selected
// by a bind mount, including paths forwarded through a macOS VM filesystem.
func mountedHostPathFor(mountInfo []byte, target string) (string, bool) {
	target = path.Clean(target)
	bestMountPoint := ""
	bestRoot := ""
	for _, line := range strings.Split(string(mountInfo), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		root := path.Clean(fields[3])
		mountPoint := path.Clean(fields[4])
		if target != mountPoint && !strings.HasPrefix(target, mountPoint+"/") {
			continue
		}
		if len(mountPoint) > len(bestMountPoint) {
			bestMountPoint = mountPoint
			bestRoot = root
		}
	}
	// The container root filesystem always contains every absolute path. It is
	// not evidence that the requested host directory was bind-mounted.
	if bestMountPoint == "" || bestMountPoint == "/" {
		return "", false
	}
	relative := strings.TrimPrefix(target, bestMountPoint)
	return path.Join(bestRoot, relative), true
}

func virtualizedMountRootMatches(mountedHostPath, hostPath string) bool {
	mountedHostPath = path.Clean(mountedHostPath)
	hostPath = path.Clean(hostPath)
	return mountedHostPath == hostPath || strings.HasSuffix(mountedHostPath, hostPath)
}

func checkPathExists(path string) StepOutcome {
	if _, err := os.Stat(path); err != nil {
		return StepOutcome{OK: false, Detail: err.Error(),
			Remediation: newStepRemediation(remediationMountPIDDirectory, map[string]string{"path": path})}
	}
	return StepOutcome{OK: true, Detail: path + " present"}
}

func okOrFail(err error, okDetail, remediationCode, raw string) StepOutcome {
	if err == nil {
		return StepOutcome{OK: true, Detail: okDetail}
	}
	detail := err.Error()
	if raw != "" {
		detail = strings.TrimSpace(raw)
	}
	return StepOutcome{OK: false, Detail: detail, Remediation: newStepRemediation(remediationCode)}
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
