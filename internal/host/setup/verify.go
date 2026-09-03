package setup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

// errMountedChecksUnsupported is returned by the platform helpers when the
// build target cannot inspect a bind mount at all. The mounted-only checks
// turn it into a warning so the operator sees why nothing was confirmed.
var errMountedChecksUnsupported = errors.New("bind-mount checks are not supported on " + runtime.GOOS)

// errInodeUnavailable is returned when the local stat result carries no inode.
var errInodeUnavailable = errors.New("could not read the local inode")

// maxConcurrentChecks bounds the SSH sessions the pipeline opens at once.
// sshd defaults MaxSessions to 10 per connection; 4 leaves headroom for the
// SFTP subsystem and any runtime command issued while a verify is running.
const maxConcurrentChecks = 4

// stepCollector gathers outcomes from concurrent checks. The JSON result is
// still a plain map, so callers see the same shape as before.
type stepCollector struct {
	mu    sync.Mutex
	steps map[string]StepOutcome
}

func newStepCollector() *stepCollector {
	return &stepCollector{steps: map[string]StepOutcome{}}
}

func (c *stepCollector) set(key string, outcome StepOutcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps[key] = outcome
}

// runBounded runs every task concurrently, at most maxConcurrentChecks at a
// time, and returns once all of them finished. Tasks are independent by
// construction: anything that needs another check's result runs after it.
func runBounded(tasks []func()) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxConcurrentChecks)
	for _, task := range tasks {
		wg.Add(1)
		slots <- struct{}{}
		go func(task func()) {
			defer wg.Done()
			defer func() { <-slots }()
			task()
		}(task)
	}
	wg.Wait()
}

// Verify runs the self-check pipeline, optionally limited to a set of groups so
// a wizard step can validate what it just configured.
func Verify(ctx context.Context, opts VerifyOptions) VerifyResult {
	steps := newStepCollector()
	r := VerifyResult{Steps: steps.steps}
	p := opts.Params.FillDefaults()

	// The same values the wizard refuses to paste into a root shell are the
	// ones the checks below stat and open, so they are validated here too.
	if err := p.ValidateSnippetValues(); err != nil {
		steps.set("parameters", StepOutcome{OK: false, Detail: err.Error(),
			Remediation: newStepRemediation(remediationCorrectParameters)})
		return r
	}

	wants := newCheckGroupFilter(opts.Groups)

	if wants(CheckGroupConnection) {
		steps.set("known_hosts_persistence", checkKnownHostsPersistence(p.ContainerKnownHostsPath))
	}

	// Every remaining check needs a working session, so this one always runs
	// and runs alone: the connection is proven before it is shared.
	connOut, err := opts.Client.Exec(ctx, "/bin/echo", "ok")
	steps.set("ssh_connect", okOrFail(err, "echo ok over ssh",
		remediationCheckSSHConnection,
		connOut))
	if err != nil {
		return r
	}

	// The remaining checks have no data dependency on each other, so they
	// share the multiplexed connection instead of paying a round trip each.
	var tasks []func()
	check := func(key string, run func() StepOutcome) {
		tasks = append(tasks, func() { steps.set(key, run()) })
	}

	if wants(CheckGroupConnection) {
		check("same_host", func() StepOutcome { return checkSameHost(ctx, opts.Client, p) })
	}

	if wants(CheckGroupPlatform) {
		check("host_platform", func() StepOutcome { return checkHostPlatform(ctx, opts.Client, p) })
		if p.IsLaunchd() {
			check("launchctl_service_loaded", func() StepOutcome { return checkLaunchdServiceLoaded(ctx, opts.Client, p) })
		} else {
			check("systemctl_is_active", func() StepOutcome { return checkSystemdUnitActive(ctx, opts.Client, p) })
			check("unit_has_execreload", func() StepOutcome { return checkSystemdExecReload(ctx, opts.Client, p) })
		}
		if p.AccessMode == settings.HostAccessModeSFTP {
			check("config_dir_writable", func() StepOutcome { return checkRemoteDirectory(ctx, opts.Client, p.HostConfigDir, true) })
			check("log_dir_readable", func() StepOutcome { return checkRemoteDirectory(ctx, opts.Client, p.HostLogDir, false) })
			check("pid_file_present", func() StepOutcome { return checkRemotePath(ctx, opts.Client, p.PIDPath) })
		} else {
			check("config_dir_writable", func() StepOutcome { return checkDirAccess(p.ContainerConfigDir, true) })
			check("config_dir_shared", func() StepOutcome {
				return checkSharedPath(ctx, opts.Client, p, p.ContainerConfigDir, p.HostConfigDir)
			})
			check("log_dir_readable", func() StepOutcome { return checkLogReadable(p.ContainerLogDir + "/access.log") })
			check("pid_file_present", func() StepOutcome { return checkPathExists(p.PIDPath) })
		}
	}

	// A target that needs no sudoers entry reports no privilege steps at all,
	// so the UI does not show checks the operator cannot act on. Both
	// privilege checks share one uid probe through the Once.
	if wants(CheckGroupPrivileges) && p.NeedsSudoers() {
		var once sync.Once
		var required bool
		sudoersNeeded := func() bool {
			once.Do(func() { required = sudoersRequired(ctx, opts.Client, p) })
			return required
		}
		tasks = append(tasks, func() {
			if sudoersNeeded() {
				steps.set("sudo_available", checkSudoAvailable(ctx, opts.Client))
			}
		})
		tasks = append(tasks, func() {
			if sudoersNeeded() {
				steps.set("sudoers_coverage", checkSudoersCoverage(ctx, opts.Client, p))
			}
		})
	}

	runBounded(tasks)

	// nginx -t is the only check with a cost and its output is what the
	// operator reads last, so it stays after everything it depends on.
	if wants(CheckGroupNginx) {
		if opts.SkipNginxT {
			// A check that did not run must not read as a pass.
			steps.set("nginx_test", StepOutcome{
				OK:     false,
				Level:  "warning",
				Detail: "skipped by user request; the configuration was not validated",
			})
		} else {
			ntOut, ntErr := opts.Client.Exec(ctx, p.NginxSbinPath, "-t")
			steps.set("nginx_test", okOrFail(ntErr, strings.TrimSpace(ntOut),
				remediationFixNginxConfig, ntOut))
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

// checkSudoAvailable proves passwordless sudo works for the SSH user.
func checkSudoAvailable(ctx context.Context, client CommandRunner) StepOutcome {
	_, err := client.Exec(ctx, "/usr/bin/sudo", "-n", "/bin/true")
	return okOrFail(err, "sudo -n true succeeded",
		remediationReviewSudoersRules, "")
}

// checkSudoersCoverage confirms every command the runtime issues under sudo is
// listed for the SSH user.
func checkSudoersCoverage(ctx context.Context, client CommandRunner, p SetupParams) StepOutcome {
	listOut, listErr := client.Exec(ctx, "/usr/bin/sudo", "-n", "-l")
	if listErr != nil {
		return StepOutcome{OK: false, Detail: listErr.Error(),
			Remediation: newStepRemediation(remediationInspectSudoPermissions)}
	}
	required := []string{
		fmt.Sprintf("%s reload %s", p.SystemctlPath, p.SystemdUnit),
		fmt.Sprintf("%s restart %s", p.SystemctlPath, p.SystemdUnit),
		fmt.Sprintf("%s -t", p.NginxSbinPath),
		fmt.Sprintf("%s -T", p.NginxSbinPath),
	}
	missing := findMissingSudoEntries(listOut, required)
	if len(missing) == 0 {
		return StepOutcome{OK: true, Detail: "all required entries present"}
	}
	return StepOutcome{OK: false,
		Detail: "missing: " + strings.Join(missing, "; "), Remediation: newStepRemediation(remediationAddMissingSudoersEntries, map[string]string{
			"path": p.SudoersFilename,
		})}
}

// checkSystemdUnitActive covers the running-state half of the systemd checks.
func checkSystemdUnitActive(ctx context.Context, client CommandRunner, p SetupParams) StepOutcome {
	isActiveOut, err := client.Exec(ctx, p.SystemctlPath, "is-active", p.SystemdUnit)
	return okOrFail(err, "is-active returned: "+strings.TrimSpace(isActiveOut),
		remediationCheckSystemdUnit, isActiveOut)
}

// checkSystemdExecReload confirms the unit can be reloaded rather than only
// restarted.
func checkSystemdExecReload(ctx context.Context, client CommandRunner, p SetupParams) StepOutcome {
	showOut, err := client.Exec(ctx, p.SystemctlPath, "show", p.SystemdUnit, "--property=ExecReload")
	if err != nil {
		return okOrFail(err, "", remediationInspectSystemdUnit, showOut)
	}
	// systemctl always prints the property name, so an undeclared ExecReload
	// shows up as an empty value rather than a missing line.
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(showOut), "ExecReload="))
	if value == "" {
		return StepOutcome{OK: false, Detail: "ExecReload is not declared in the unit",
			Remediation: newStepRemediation(remediationRestartWithoutExecReload)}
	}
	return StepOutcome{OK: true, Detail: "ExecReload is declared"}
}

// checkLaunchdServiceLoaded looks the service up in the SSH user's own
// launchd domain, which needs the uid first.
func checkLaunchdServiceLoaded(ctx context.Context, client CommandRunner, p SetupParams) StepOutcome {
	uidOut, err := client.Exec(ctx, "/usr/bin/id", "-u")
	if err != nil {
		return StepOutcome{OK: false, Detail: err.Error(), Remediation: newStepRemediation(remediationConfirmHomebrewServiceOwner)}
	}
	uid := strings.TrimSpace(uidOut)
	parsedUID, err := strconv.ParseUint(uid, 10, 32)
	if err != nil || parsedUID == 0 {
		return StepOutcome{OK: false, Detail: "invalid remote user id: " + uid}
	}
	target := "gui/" + uid + "/" + p.LaunchdService
	out, err := client.Exec(ctx, p.LaunchctlPath, "print", target)
	return okOrFail(err, target+" is loaded",
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
	if err := dirAccessible(path, writable); err != nil {
		if errors.Is(err, errMountedChecksUnsupported) {
			return StepOutcome{OK: false, Level: "warning",
				Detail:      path + " exists, but " + err.Error(),
				Remediation: newStepRemediation(remediationVerifyBindMount)}
		}
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
	localIno, err := localInode(info)
	if err != nil {
		return StepOutcome{OK: false, Level: "warning", Detail: err.Error(),
			Remediation: newStepRemediation(remediationVerifyBindMount)}
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
	if remote != localIno {
		return StepOutcome{OK: false,
			Detail: fmt.Sprintf("%s and host %s are different directories (inode %d vs %d)",
				containerPath, hostPath, localIno, remote),
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
