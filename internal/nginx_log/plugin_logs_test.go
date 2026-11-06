package nginx_log

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginLogFilesListsConfigAndDefaultPaths(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	logDir := t.TempDir()
	useLogDirWhiteList(t, logDir)
	siteLog := writeLogFile(t, logDir, "site.access.log")
	siteError := writeLogFile(t, logDir, "site.error.log")
	defaultAccess := writeLogFile(t, logDir, "access.log")
	outside := filepath.Join(t.TempDir(), "outside.log")

	AddLogPath(siteLog, "access", "site.access.log", "/etc/nginx/sites-enabled/site.conf")
	AddLogPath(siteError, "error", "site.error.log", "/etc/nginx/sites-enabled/site.conf")
	AddLogPath(outside, "access", "outside.log", "/etc/nginx/nginx.conf")
	AddLogPath(filepath.Join(logDir, "odd.log"), "audit", "odd.log", "/etc/nginx/nginx.conf")

	defaultLogRegistryMutex.Lock()
	defaultLogRegistry[defaultAccess] = &NginxLogCache{Path: defaultAccess, Type: "access", Name: "access.log"}
	// A path that a directive also names is listed once, as a config path.
	defaultLogRegistry[siteLog] = &NginxLogCache{Path: siteLog, Type: "access", Name: "site.access.log"}
	defaultLogRegistryMutex.Unlock()

	assert.Equal(t, []PluginLogFile{
		{Path: defaultAccess, Type: "access", Source: PluginLogSourceDefault},
		{Path: siteLog, Type: "access", Source: PluginLogSourceConfig, ConfigFile: "/etc/nginx/sites-enabled/site.conf"},
		{Path: siteError, Type: "error", Source: PluginLogSourceConfig, ConfigFile: "/etc/nginx/sites-enabled/site.conf"},
	}, PluginLogFiles())
}

func TestPluginLogFilesIsNeverNil(t *testing.T) {
	resetConfigLogRegistry(t)
	resetDefaultLogRegistry(t)

	files := PluginLogFiles()
	require.NotNil(t, files)
	assert.Empty(t, files)
}

func TestSubscribeLogScan(t *testing.T) {
	var first, second int
	unsubscribeFirst := SubscribeLogScan(func() { first++ })
	unsubscribeSecond := SubscribeLogScan(func() { second++ })

	notifyLogScan()
	assert.Equal(t, 1, first)
	assert.Equal(t, 1, second)

	unsubscribeFirst()
	notifyLogScan()
	assert.Equal(t, 1, first)
	assert.Equal(t, 2, second)

	unsubscribeSecond()
	notifyLogScan()
	assert.Equal(t, 2, second)
}
