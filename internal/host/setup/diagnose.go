package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

// HostDiagnosis describes the target platform and the nginx installation
// discovered through read-only SSH commands.
type HostDiagnosis struct {
	OS             string          `json:"os"`
	Arch           string          `json:"arch,omitempty"`
	ServiceManager string          `json:"service_manager,omitempty"`
	SystemctlPath  string          `json:"systemctl_path,omitempty"`
	LaunchctlPath  string          `json:"launchctl_path,omitempty"`
	SystemdUnit    string          `json:"systemd_unit,omitempty"`
	LaunchdService string          `json:"launchd_service,omitempty"`
	HomebrewPrefix string          `json:"homebrew_prefix,omitempty"`
	Nginx          *NginxDiscovery `json:"nginx,omitempty"`
	Warnings       []string        `json:"warnings,omitempty"`
}

// DiagnoseHost detects the target OS before probing platform-specific nginx
// locations. It does not persist settings or mutate the SSH host.
func DiagnoseHost(ctx context.Context, executor remoteExecutor, params SetupParams) (*HostDiagnosis, error) {
	osOutput, err := executor.Exec(ctx, "/usr/bin/uname", "-s")
	if err != nil {
		return nil, fmt.Errorf("detect target operating system: %w", err)
	}

	result := &HostDiagnosis{OS: strings.TrimSpace(osOutput)}
	if archOutput, archErr := executor.Exec(ctx, "/usr/bin/uname", "-m"); archErr == nil {
		result.Arch = strings.TrimSpace(archOutput)
	} else {
		result.Warnings = append(result.Warnings, "could not detect target architecture: "+archErr.Error())
	}

	detectedParams := params
	switch result.OS {
	case "Linux":
		result.ServiceManager = settings.HostServiceManagerSystemd
		detectedParams.ServiceManager = result.ServiceManager
		// A probed path is reported. A guess is only used internally to keep the
		// nginx discovery going, so the wizard never labels it as detected.
		systemctlPath := detectExecutable(ctx, executor, params.SystemctlPath, "/bin/systemctl", "/usr/bin/systemctl")
		if systemctlPath == "" {
			result.Warnings = append(result.Warnings, "systemctl was not detected at a standard path")
			detectedParams.SystemctlPath = "/bin/systemctl"
		} else {
			result.SystemctlPath = systemctlPath
			detectedParams.SystemctlPath = systemctlPath
		}
		result.SystemdUnit = detectSystemdUnit(ctx, executor, detectedParams.SystemctlPath, params.SystemdUnit)
		if result.SystemdUnit == "" {
			result.Warnings = append(result.Warnings, "no loaded nginx systemd unit was found; enter the unit name manually")
		} else {
			detectedParams.SystemdUnit = result.SystemdUnit
		}
	case "Darwin":
		result.ServiceManager = settings.HostServiceManagerLaunchd
		detectedParams.ServiceManager = result.ServiceManager
		result.LaunchctlPath = detectExecutable(ctx, executor, params.LaunchctlPath, "/bin/launchctl")
		if result.LaunchctlPath == "" {
			result.Warnings = append(result.Warnings, "launchctl was not detected at a standard path")
			detectedParams.LaunchctlPath = "/bin/launchctl"
		} else {
			detectedParams.LaunchctlPath = result.LaunchctlPath
		}
		result.HomebrewPrefix = detectHomebrewInstallationPrefix(ctx, executor)
		result.LaunchdService = detectLaunchdService(ctx, executor, result.HomebrewPrefix, detectedParams.LaunchctlPath, params.LaunchdService)
		if result.LaunchdService == "" {
			result.Warnings = append(result.Warnings, "no loaded nginx launchd service was found; enter the service label manually")
		} else {
			detectedParams.LaunchdService = result.LaunchdService
		}
		if result.HomebrewPrefix == "" {
			result.Warnings = append(result.Warnings, "Homebrew was not detected at /opt/homebrew or /usr/local")
		} else if detectedParams.NginxSbinPath == "" {
			// Only seed the Homebrew location. An explicitly configured binary,
			// such as a source build, must stay the first discovery candidate.
			detectedParams.NginxSbinPath = path.Join(result.HomebrewPrefix, "opt/nginx/bin/nginx")
		}
	default:
		result.Warnings = append(result.Warnings, "unsupported target operating system: "+result.OS)
		return result, nil
	}

	nginxResult, nginxErr := DiscoverNginx(ctx, executor, detectedParams)
	if nginxErr != nil {
		result.Warnings = append(result.Warnings, nginxErr.Error())
	} else {
		result.Nginx = nginxResult
		if result.HomebrewPrefix == "" {
			result.HomebrewPrefix = nginxResult.HomebrewPrefix
		}
	}

	return result, nil
}

// knownLaunchdLabel matches the labels Homebrew and the nginx project use.
var knownLaunchdLabel = regexp.MustCompile(`^([a-z0-9.]+\.)?mxcl\.nginx$|^org\.nginx\.`)

// knownSystemdUnits are the unit names nginx ships under across distributions
// and forks. The configured value is probed first so a custom unit still wins.
var knownSystemdUnits = []string{"nginx.service", "openresty.service", "tengine.service"}

// detectSystemdUnit returns the first nginx unit systemd reports as loaded.
func detectSystemdUnit(ctx context.Context, executor remoteExecutor, systemctlPath, configured string) string {
	seen := make(map[string]struct{})
	candidates := make([]string, 0, len(knownSystemdUnits)+1)
	for _, candidate := range append([]string{strings.TrimSpace(configured)}, knownSystemdUnits...) {
		if candidate == "" {
			continue
		}
		if !strings.HasSuffix(candidate, ".service") {
			candidate += ".service"
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	for _, candidate := range candidates {
		out, err := executor.Exec(ctx, systemctlPath, "show", candidate, "--property=LoadState")
		if err != nil {
			continue
		}
		if strings.Contains(out, "LoadState=loaded") {
			return candidate
		}
	}
	return ""
}

// detectLaunchdServiceFromBrew asks Homebrew for the label it manages nginx
// under. Homebrew is authoritative here; launchctl only exposes a name that
// happens to follow a convention.
func detectLaunchdServiceFromBrew(ctx context.Context, executor remoteExecutor, homebrewPrefix string) string {
	if homebrewPrefix == "" {
		return ""
	}
	out, err := executor.Exec(ctx, path.Join(homebrewPrefix, "bin/brew"), "services", "info", "nginx", "--json")
	if err != nil {
		return ""
	}
	// Exec returns stdout and stderr combined, so skip anything brew printed
	// before the JSON document.
	start := strings.Index(out, "[")
	if start < 0 {
		return ""
	}
	var entries []struct {
		ServiceName string `json:"service_name"`
	}
	if err := json.Unmarshal([]byte(out[start:]), &entries); err != nil {
		return ""
	}
	for _, entry := range entries {
		if name := strings.TrimSpace(entry.ServiceName); name != "" {
			return name
		}
	}
	return ""
}

// detectLaunchdService returns the launchd label that owns nginx. Homebrew is
// asked first; the launchctl listing is only a fallback.
func detectLaunchdService(ctx context.Context, executor remoteExecutor, homebrewPrefix, launchctlPath, configured string) string {
	if name := detectLaunchdServiceFromBrew(ctx, executor, homebrewPrefix); name != "" {
		return name
	}

	out, err := executor.Exec(ctx, launchctlPath, "list")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		label := fields[len(fields)-1]
		// Only an exact match is trusted. A loose "contains nginx" match would
		// happily return an exporter or a log rotator, and the service checks
		// would then pass against the wrong job.
		if label == strings.TrimSpace(configured) || knownLaunchdLabel.MatchString(label) {
			return label
		}
	}
	return ""
}

func detectExecutable(ctx context.Context, executor remoteExecutor, candidates ...string) string {
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if !path.IsAbs(candidate) {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := executor.Exec(ctx, candidate, "--version"); err == nil {
			return candidate
		}
	}
	return ""
}

func detectHomebrewInstallationPrefix(ctx context.Context, executor remoteExecutor) string {
	for _, brew := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if output, err := executor.Exec(ctx, brew, "--prefix"); err == nil {
			prefix := strings.TrimSpace(output)
			if path.IsAbs(prefix) {
				return path.Clean(prefix)
			}
		}
	}
	return ""
}
