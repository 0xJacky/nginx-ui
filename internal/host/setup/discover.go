package setup

import (
	"context"
	"path"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
)

type remoteExecutor interface {
	Exec(ctx context.Context, name string, args ...string) (string, error)
}

// NginxDiscovery describes paths compiled into the nginx binary on the SSH host.
type NginxDiscovery struct {
	Version        string `json:"version"`
	ExecutablePath string `json:"executable_path"`
	Prefix         string `json:"prefix,omitempty"`
	ConfigPath     string `json:"config_path,omitempty"`
	ConfigDir      string `json:"config_dir,omitempty"`
	PIDPath        string `json:"pid_path,omitempty"`
	AccessLogPath  string `json:"access_log_path,omitempty"`
	ErrorLogPath   string `json:"error_log_path,omitempty"`
	LogDir         string `json:"log_dir,omitempty"`
	HomebrewPrefix string `json:"homebrew_prefix,omitempty"`
	DocumentRoot   string `json:"document_root,omitempty"`
}

var nginxVersionPattern = regexp.MustCompile(`(?m)^nginx version:\s*([^\s]+)`)

// DiscoverNginx finds the active nginx installation without mutating the host.
// It probes the caller-supplied path first, then Homebrew and common Unix paths.
func DiscoverNginx(ctx context.Context, executor remoteExecutor, params SetupParams) (*NginxDiscovery, error) {
	p := params.FillDefaults()
	candidates := nginxCandidates(ctx, executor, p)
	for _, candidate := range candidates {
		out, err := executor.Exec(ctx, candidate, "-V")
		if err != nil {
			continue
		}
		result := parseNginxVersionOutput(candidate, out, p)
		if result.Version != "" {
			return result, nil
		}
	}
	return nil, cosy.WrapErrorWithParams(ErrDiscoveryFailed, strings.Join(candidates, ", "))
}

func nginxCandidates(ctx context.Context, executor remoteExecutor, p SetupParams) []string {
	var candidates []string
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if !path.IsAbs(candidate) || path.Clean(candidate) != candidate {
			return
		}
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	add(p.NginxSbinPath)
	if p.IsLaunchd() {
		for _, brew := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
			if prefix, err := executor.Exec(ctx, brew, "--prefix", "nginx"); err == nil {
				add(path.Join(strings.TrimSpace(prefix), "bin/nginx"))
			}
		}
		add("/opt/homebrew/bin/nginx")
		add("/opt/homebrew/opt/nginx/bin/nginx")
		add("/usr/local/bin/nginx")
		add("/usr/local/opt/nginx/bin/nginx")
	} else {
		add("/usr/sbin/nginx")
		add("/usr/local/sbin/nginx")
		add("/usr/local/nginx/sbin/nginx")
	}
	if discovered, err := executor.Exec(ctx, "/usr/bin/which", "nginx"); err == nil {
		add(strings.TrimSpace(discovered))
	}
	return candidates
}

func parseNginxVersionOutput(executablePath, output string, p SetupParams) *NginxDiscovery {
	result := &NginxDiscovery{ExecutablePath: executablePath}
	if match := nginxVersionPattern.FindStringSubmatch(output); len(match) == 2 {
		result.Version = match[1]
	}
	result.Prefix = nginx.ExtractConfigureArg(output, "--prefix")
	result.ConfigPath = resolveNginxPath(result.Prefix, nginx.ExtractConfigureArg(output, "--conf-path"))
	if result.ConfigPath != "" {
		result.ConfigDir = path.Dir(result.ConfigPath)
	}
	result.PIDPath = resolveNginxPath(result.Prefix, nginx.ExtractConfigureArg(output, "--pid-path"))
	result.AccessLogPath = resolveNginxPath(result.Prefix, nginx.ExtractConfigureArg(output, "--http-log-path"))
	result.ErrorLogPath = resolveNginxPath(result.Prefix, nginx.ExtractConfigureArg(output, "--error-log-path"))
	if result.AccessLogPath != "" && result.ErrorLogPath != "" && path.Dir(result.AccessLogPath) == path.Dir(result.ErrorLogPath) {
		result.LogDir = path.Dir(result.AccessLogPath)
	} else if result.AccessLogPath != "" {
		result.LogDir = path.Dir(result.AccessLogPath)
	} else if result.ErrorLogPath != "" {
		result.LogDir = path.Dir(result.ErrorLogPath)
	}
	if p.IsLaunchd() {
		result.HomebrewPrefix = detectHomebrewPrefix(executablePath, result.ConfigDir)
		if result.HomebrewPrefix != "" {
			result.DocumentRoot = path.Join(result.HomebrewPrefix, "var/www")
		}
	}
	return result
}

// resolveNginxPath joins a relative compile-time path onto the nginx prefix.
// The SSH host is always POSIX, so this deliberately uses path rather than the
// local filepath rules internal/nginx applies to its own target.
func resolveNginxPath(prefix, value string) string {
	if value == "" || path.IsAbs(value) {
		return value
	}
	if path.IsAbs(prefix) {
		return path.Join(prefix, value)
	}
	return value
}

func detectHomebrewPrefix(executablePath, configDir string) string {
	const configSuffix = "/etc/nginx"
	if strings.HasSuffix(configDir, configSuffix) {
		return strings.TrimSuffix(configDir, configSuffix)
	}
	for _, suffix := range []string{"/opt/nginx/bin/nginx", "/bin/nginx"} {
		if strings.HasSuffix(executablePath, suffix) {
			return strings.TrimSuffix(executablePath, suffix)
		}
	}
	return ""
}
