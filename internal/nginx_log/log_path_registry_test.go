package nginx_log

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// resetConfigLogRegistry isolates a test from log paths registered by other
// tests and restores the previous content afterwards.
func resetConfigLogRegistry(t *testing.T) {
	t.Helper()

	configLogRegistryMutex.Lock()
	previous := configLogRegistry
	configLogRegistry = make(map[string]*NginxLogCache)
	configLogRegistryMutex.Unlock()

	t.Cleanup(func() {
		configLogRegistryMutex.Lock()
		configLogRegistry = previous
		configLogRegistryMutex.Unlock()
	})
}

func logPaths(logs []*NginxLogCache) []string {
	paths := make([]string, 0, len(logs))
	for _, log := range logs {
		paths = append(paths, log.Path)
	}
	sort.Strings(paths)
	return paths
}

// newDiscoverableAccessLog creates a whitelisted access log file so
// utils.IsValidLogPath accepts the directive path. It also neutralises the nginx
// default log settings, so the machine's own nginx installation cannot register
// extra log paths behind the test's back.
func newDiscoverableAccessLog(t *testing.T) string {
	t.Helper()

	logDir := t.TempDir()
	logPath := writeLogFile(t, logDir, "access.log")

	useLogDirWhiteList(t, logDir)
	useDefaultLogSettings(t,
		makeDirectory(t, logDir, "no-access-log"),
		makeDirectory(t, logDir, "no-error-log"))

	return logPath
}

// TestScanForLogDirectivesRegistersPaths checks that a directive found by the
// configuration scan becomes a listed log path.
func TestScanForLogDirectivesRegistersPaths(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logPath := newDiscoverableAccessLog(t)
	configPath := "/etc/nginx/conf.d/early.conf"

	require.NoError(t, scanForLogDirectives(configPath,
		[]byte("server {\n    access_log "+logPath+";\n}\n")))
	require.Equal(t, []string{logPath}, logPaths(GetAllLogPaths()))
	require.Equal(t, []string{logPath}, logPaths(GetAllLogPathsGrouped()))
}

// TestRemoveLogPathsFromConfigClearsRegistry makes sure the registry does not
// keep serving log paths whose directive was removed from the configuration.
func TestRemoveLogPathsFromConfigClearsRegistry(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logPath := newDiscoverableAccessLog(t)
	configPath := "/etc/nginx/conf.d/removable.conf"

	require.NoError(t, scanForLogDirectives(configPath,
		[]byte("server {\n    access_log "+logPath+";\n}\n")))
	require.Equal(t, []string{logPath}, logPaths(GetAllLogPaths()))

	// The directive is gone from the config file: the scanner replays the file
	// with content that no longer declares it.
	require.NoError(t, scanForLogDirectives(configPath, []byte("server {\n}\n")))
	require.Empty(t, GetAllLogPaths())
	require.Empty(t, GetAllLogPathsGrouped())
}

// TestGetAllLogPathsGroupedFoldsRotatedFiles checks that rotated files share
// the group of their main log and that filters apply to the group.
func TestGetAllLogPathsGroupedFoldsRotatedFiles(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	AddLogPath("/var/log/nginx/access.log", "access", "access.log", "/etc/nginx/nginx.conf")
	AddLogPath("/var/log/nginx/access.log.1", "access", "access.log.1", "/etc/nginx/nginx.conf")
	AddLogPath("/var/log/nginx/error.log", "error", "error.log", "/etc/nginx/nginx.conf")

	groups := GetAllLogPathsGrouped()
	require.Equal(t, []string{"/var/log/nginx/access.log", "/var/log/nginx/error.log"}, logPaths(groups))
	require.Equal(t, "access.log", groups[0].Name)

	errorsOnly := GetAllLogPathsGrouped(func(log *NginxLogCache) bool { return log.Type == "error" })
	require.Equal(t, []string{"/var/log/nginx/error.log"}, logPaths(errorsOnly))
}
