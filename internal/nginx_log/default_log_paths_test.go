package nginx_log

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

// resetDefaultLogRegistry isolates a test from the default log paths registered
// by other tests and restores the previous content afterwards.
func resetDefaultLogRegistry(t *testing.T) {
	t.Helper()

	defaultLogRegistryMutex.Lock()
	previous := defaultLogRegistry
	defaultLogRegistry = make(map[string]*NginxLogCache)
	defaultLogRegistryMutex.Unlock()

	t.Cleanup(func() {
		defaultLogRegistryMutex.Lock()
		defaultLogRegistry = previous
		defaultLogRegistryMutex.Unlock()
	})
}

// useDefaultLogSettings points settings.NginxSettings at the given default log
// paths and restores the previous values afterwards. Both paths are always set:
// nginx.GetAccessLogPath and nginx.GetErrorLogPath fall back to `nginx -V` when
// their setting is empty, so leaving one blank would let the real nginx
// installation of the machine running the test leak into the assertions.
func useDefaultLogSettings(t *testing.T, accessLogPath, errorLogPath string) {
	t.Helper()

	originalAccessLogPath := settings.NginxSettings.AccessLogPath
	originalErrorLogPath := settings.NginxSettings.ErrorLogPath
	t.Cleanup(func() {
		settings.NginxSettings.AccessLogPath = originalAccessLogPath
		settings.NginxSettings.ErrorLogPath = originalErrorLogPath
	})

	settings.NginxSettings.AccessLogPath = accessLogPath
	settings.NginxSettings.ErrorLogPath = errorLogPath
}

// useLogDirWhiteList restricts the log directory whitelist to the given
// directories for the duration of the test.
func useLogDirWhiteList(t *testing.T, dirs ...string) {
	t.Helper()

	originalWhiteList := settings.NginxSettings.LogDirWhiteList
	t.Cleanup(func() {
		settings.NginxSettings.LogDirWhiteList = originalWhiteList
	})

	settings.NginxSettings.LogDirWhiteList = dirs
}

// writeLogFile creates an empty but readable log file and returns its path.
func writeLogFile(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("\n"), 0o600))
	return path
}

// makeDirectory creates a directory and returns its path. Pointing a default log
// setting at it is how a test disables that default: utils.IsValidLogPath only
// accepts regular files.
func makeDirectory(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(path, 0o755))
	return path
}

// groupsByPath indexes the grouped log list by path for direct field assertions.
func groupsByPath(logs []*NginxLogCache) map[string]*NginxLogCache {
	byPath := make(map[string]*NginxLogCache, len(logs))
	for _, log := range logs {
		byPath[log.Path] = log
	}
	return byPath
}

// isDefaultLogPath reports whether the path is one of the nginx default logs.
func isDefaultLogPath(path string) bool {
	defaultLogRegistryMutex.RLock()
	defer defaultLogRegistryMutex.RUnlock()

	_, ok := defaultLogRegistry[path]
	return ok
}

// isConfigLogPath reports whether the path is declared by a configuration file.
func isConfigLogPath(path string) bool {
	configLogRegistryMutex.RLock()
	defer configLogRegistryMutex.RUnlock()

	_, ok := configLogRegistry[path]
	return ok
}

// TestDefaultLogPathsAreListedWithoutDirectives is the regression test for
// issue #1787.
//
// The reporter runs the Homebrew nginx build, whose shipped nginx.conf has every
// access_log directive commented out. He configured the real log paths through
// nginx.AccessLogPath in app.ini, which made the log preview work, but the log
// list stayed empty: the only source of listed log paths was
// scanForLogDirectives, and it can only find paths that a directive spells out.
func TestDefaultLogPathsAreListedWithoutDirectives(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logDir := t.TempDir()
	accessLogPath := writeLogFile(t, logDir, "access.log")
	errorLogPath := writeLogFile(t, logDir, "error.log")

	useLogDirWhiteList(t, logDir)
	useDefaultLogSettings(t, accessLogPath, errorLogPath)

	// The configuration scan runs over the Homebrew nginx.conf and finds nothing,
	// because both access_log lines are comments.
	require.NoError(t, scanForLogDirectives("/opt/homebrew/etc/nginx/nginx.conf", []byte(
		"http {\n"+
			"    #access_log  logs/access.log  main;\n"+
			"    server {\n"+
			"        #access_log  logs/host.access.log  main;\n"+
			"    }\n"+
			"}\n")))
	require.Empty(t, GetAllLogPathsGrouped(),
		"a configuration whose access_log directives are all commented out discovers no log path")

	// Resolving the nginx defaults is what makes the log groups appear.
	require.Equal(t, 2, RefreshDefaultLogPaths())
	require.Equal(t, []string{accessLogPath, errorLogPath},
		logPaths(GetAllLogPathsGrouped()))

	groups := groupsByPath(GetAllLogPathsGrouped())
	require.Equal(t, logTypeAccess, groups[accessLogPath].Type,
		"the default access log must be typed as an access log")
	require.Equal(t, logTypeError, groups[errorLogPath].Type)

	// No configuration file owns these entries, so no rescan can take them away.
	require.Empty(t, groups[accessLogPath].ConfigFile)
	require.True(t, isDefaultLogPath(accessLogPath))
	require.False(t, isConfigLogPath(accessLogPath))
}

// TestRefreshDefaultLogPathsRejectsInvalidPaths makes sure the default paths go
// through the same validation as a path taken from a directive. Both settings
// accept arbitrary values, and the whitelist check must not be skipped just
// because it usually passes for the defaults.
func TestRefreshDefaultLogPathsRejectsInvalidPaths(t *testing.T) {
	t.Run("not a regular file", func(t *testing.T) {
		resetConfigLogRegistry(t)
		resetDefaultLogRegistry(t)

		logDir := t.TempDir()
		useLogDirWhiteList(t, logDir)
		useDefaultLogSettings(t,
			makeDirectory(t, logDir, "access.log"),
			makeDirectory(t, logDir, "error.log"))

		require.Zero(t, RefreshDefaultLogPaths())
		require.Empty(t, GetAllLogPaths())
		require.Empty(t, GetAllLogPathsGrouped())
	})

	t.Run("symlink resolving outside the whitelist", func(t *testing.T) {
		resetConfigLogRegistry(t)
		resetDefaultLogRegistry(t)

		logDir := t.TempDir()
		outsideDir := t.TempDir()
		outsidePath := writeLogFile(t, outsideDir, "smuggled.log")

		accessLogPath := filepath.Join(logDir, "access.log")
		require.NoError(t, os.Symlink(outsidePath, accessLogPath))

		useLogDirWhiteList(t, logDir)
		useDefaultLogSettings(t, accessLogPath, makeDirectory(t, logDir, "no-error-log"))

		require.Zero(t, RefreshDefaultLogPaths(),
			"a default log path whose symlink target escapes the whitelist must be rejected")
		require.Empty(t, GetAllLogPaths())
	})
}

// TestDefaultLogPathNotDuplicatedByDirective covers a path that is both the
// nginx default access log and the target of an access_log directive. It must
// yield a single log group, and removing the directive must not take the default
// away.
func TestDefaultLogPathNotDuplicatedByDirective(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logDir := t.TempDir()
	accessLogPath := writeLogFile(t, logDir, "access.log")

	useLogDirWhiteList(t, logDir)
	useDefaultLogSettings(t, accessLogPath, makeDirectory(t, logDir, "no-error-log"))

	require.Equal(t, 1, RefreshDefaultLogPaths())

	configPath := "/etc/nginx/conf.d/default-access-log.conf"
	require.NoError(t, scanForLogDirectives(configPath,
		[]byte("server {\n    access_log "+accessLogPath+" main;\n}\n")))

	require.Equal(t, []string{accessLogPath}, logPaths(GetAllLogPaths()),
		"a path declared by a directive and used as the default log must be registered once")
	require.Equal(t, []string{accessLogPath}, logPaths(GetAllLogPathsGrouped()))
	require.Len(t, GetAllLogPaths(), 1)

	// The directive is commented out again. The path is still the default access
	// log, so it must survive the rescan that drops the directive.
	require.NoError(t, scanForLogDirectives(configPath, []byte("server {\n}\n")))
	require.Equal(t, []string{accessLogPath}, logPaths(GetAllLogPaths()))
	require.Equal(t, []string{accessLogPath}, logPaths(GetAllLogPathsGrouped()))
}

// TestRefreshDefaultLogPathsFollowsSettingsChange checks that re-resolving picks
// up a new nginx.AccessLogPath and forgets the previous one. The post-scan
// callback re-runs this on every configuration scan sweep, so the change takes
// effect without a restart.
func TestRefreshDefaultLogPathsFollowsSettingsChange(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logDir := t.TempDir()
	firstLogPath := writeLogFile(t, logDir, "first.log")
	secondLogPath := writeLogFile(t, logDir, "second.log")
	noErrorLog := makeDirectory(t, logDir, "no-error-log")

	useLogDirWhiteList(t, logDir)
	useDefaultLogSettings(t, firstLogPath, noErrorLog)

	require.Equal(t, 1, RefreshDefaultLogPaths())
	require.Equal(t, []string{firstLogPath}, logPaths(GetAllLogPaths()))

	settings.NginxSettings.AccessLogPath = secondLogPath
	require.Equal(t, 1, RefreshDefaultLogPaths())
	require.Equal(t, []string{secondLogPath}, logPaths(GetAllLogPaths()))
	require.False(t, isDefaultLogPath(firstLogPath))
}

// TestRefreshDefaultLogPathsKeepsPathStillDeclaredByDirective makes sure that
// dropping a stale default never removes a path an access_log directive still
// declares.
func TestRefreshDefaultLogPathsKeepsPathStillDeclaredByDirective(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logDir := t.TempDir()
	sharedLogPath := writeLogFile(t, logDir, "shared.log")
	newDefaultLogPath := writeLogFile(t, logDir, "new-default.log")
	noErrorLog := makeDirectory(t, logDir, "no-error-log")

	useLogDirWhiteList(t, logDir)
	useDefaultLogSettings(t, sharedLogPath, noErrorLog)

	require.Equal(t, 1, RefreshDefaultLogPaths())
	require.NoError(t, scanForLogDirectives("/etc/nginx/conf.d/shared.conf",
		[]byte("server {\n    access_log "+sharedLogPath+" main;\n}\n")))

	settings.NginxSettings.AccessLogPath = newDefaultLogPath
	require.Equal(t, 1, RefreshDefaultLogPaths())

	require.Equal(t, []string{newDefaultLogPath, sharedLogPath},
		logPaths(GetAllLogPaths()),
		"the previous default is still declared by a directive and must stay registered")
}

// TestRemoveLogPathsFromConfigIgnoresDefaultMarker guards the marker stored in
// ConfigFile for the nginx default logs. It has to be a value no configuration
// file can ever carry, and a removal request for it must be refused instead of
// wiping every default at once.
func TestRemoveLogPathsFromConfigIgnoresDefaultMarker(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logDir := t.TempDir()
	accessLogPath := writeLogFile(t, logDir, "access.log")

	useLogDirWhiteList(t, logDir)
	useDefaultLogSettings(t, accessLogPath, makeDirectory(t, logDir, "no-error-log"))

	require.Equal(t, 1, RefreshDefaultLogPaths())

	RemoveLogPathsFromConfig(defaultLogConfigFile)
	require.Equal(t, []string{accessLogPath}, logPaths(GetAllLogPaths()))
}
