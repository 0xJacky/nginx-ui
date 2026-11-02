package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLogLevelMatchesLevelConstants(t *testing.T) {
	assert.Equal(t, Debug, GetLogLevel("nginx: [debug] test"))
	assert.Equal(t, Info, GetLogLevel("nginx: [info] test"))
	assert.Equal(t, Notice, GetLogLevel("nginx: [notice] configuration file test is successful"))
	assert.Equal(t, Warn, GetLogLevel("nginx: [warn] conflicting server name"))
	assert.Equal(t, Error, GetLogLevel("nginx: [error] invalid PID number"))
	assert.Equal(t, Crit, GetLogLevel("nginx: [crit] test"))
	assert.Equal(t, Alert, GetLogLevel("nginx: [alert] test"))
	assert.Equal(t, Emerg, GetLogLevel("nginx: [emerg] bind() to 0.0.0.0:80 failed"))
	assert.Equal(t, Unknown, GetLogLevel("nginx: something else"))
}

func TestGetLogLevelIgnoresLevelNamesInsideTheMessage(t *testing.T) {
	assert.Equal(t, Warn, GetLogLevel(`nginx: [warn] conflicting server name "alertmanager.example.com" on 0.0.0.0:80, ignored`))
	assert.Equal(t, Warn, GetLogLevel(`nginx: [warn] the "listen ... http2" directive is deprecated, use the "http2" directive instead in /etc/nginx/sites-enabled/error-pages.conf:2`))
	assert.Equal(t, Notice, GetLogLevel("nginx: [notice] signal process started, error_log is /var/log/nginx/critical.log"))
	assert.Equal(t, Unknown, GetLogLevel("nginx: the configuration file /etc/nginx/nginx.conf syntax is ok, see /var/log/nginx/error.log"))
	assert.Equal(t, Emerg, GetLogLevel("nginx: [warn] low address space\nnginx: [emerg] bind() to 0.0.0.0:80 failed"))
}

func TestControlResultIsNotErrorForWarnOutputNamingAnAlertHost(t *testing.T) {
	result := Control(func() (string, error) {
		return `nginx: [warn] conflicting server name "alertmanager.example.com" on 0.0.0.0:80, ignored
nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
nginx: configuration file /etc/nginx/nginx.conf test is successful`, nil
	})

	assert.False(t, result.IsError())
	assert.Equal(t, Warn, result.GetLevel())
}

func TestControlResultIsErrorForErrorLevelOutput(t *testing.T) {
	result := Control(func() (string, error) {
		return "nginx: [error] invalid PID number in /run/nginx.pid", nil
	})

	assert.True(t, result.IsError())
	assert.Equal(t, Error, result.GetLevel())
}
