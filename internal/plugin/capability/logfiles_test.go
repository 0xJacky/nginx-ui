package capability

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
)

func TestLogFileSourceMapsTheRegistryEntries(t *testing.T) {
	var notify func()
	unsubscribed := false
	source := NewLogFileSource(
		func() []nginx_log.PluginLogFile {
			return []nginx_log.PluginLogFile{
				{Path: "/var/log/nginx/access.log", Type: "access", Source: nginx_log.PluginLogSourceDefault},
				{Path: "/var/log/nginx/a.error.log", Type: "error", Source: nginx_log.PluginLogSourceConfig, ConfigFile: "/etc/nginx/conf.d/a.conf"},
			}
		},
		func(fn func()) func() {
			notify = fn
			return func() { unsubscribed = true }
		})

	assert.Equal(t, []protocol.HostLogFile{
		{Path: "/var/log/nginx/access.log", Type: "access", Source: "default"},
		{Path: "/var/log/nginx/a.error.log", Type: "error", Source: "config", ConfigFile: "/etc/nginx/conf.d/a.conf"},
	}, source.LogFiles())

	called := false
	unsubscribe := source.SubscribeScan(func() { called = true })
	notify()
	assert.True(t, called)
	unsubscribe()
	assert.True(t, unsubscribed)
}

func TestLogFileSourceListIsNotNil(t *testing.T) {
	source := NewLogFileSource(func() []nginx_log.PluginLogFile { return nil }, nil)
	assert.NotNil(t, source.LogFiles())
	assert.Empty(t, source.LogFiles())
}
