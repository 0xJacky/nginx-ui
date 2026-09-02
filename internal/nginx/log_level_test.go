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

func TestControlResultIsErrorForErrorLevelOutput(t *testing.T) {
	result := Control(func() (string, error) {
		return "nginx: [error] invalid PID number in /run/nginx.pid", nil
	})

	assert.True(t, result.IsError())
	assert.Equal(t, Error, result.GetLevel())
}
