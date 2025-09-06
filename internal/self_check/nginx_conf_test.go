package self_check

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func TestCheckNginxConfIncludeSites(t *testing.T) {
	// test ok
	logger.Init("debug")
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/ok.conf"
	var result *cosy.Error
	errors.As(CheckNginxConfIncludeSites(), &result)
	assert.Nil(t, result)

	// test 4041 nginx.conf not found
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/4041.conf"
	errors.As(CheckNginxConfIncludeSites(), &result)
	assert.Equal(t, int32(40402), result.Code)

	// test 5001 nginx.conf parse error
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/5001.conf"
	errors.As(CheckNginxConfIncludeSites(), &result)
	assert.Equal(t, int32(50001), result.Code)

	// test 4042 nginx.conf no http block
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/no-http-block.conf"
	errors.As(CheckNginxConfIncludeSites(), &result)
	assert.Equal(t, int32(40403), result.Code)

	// test 4043 nginx.conf not include sites-enabled
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/no-http-sites-enabled.conf"
	errors.As(CheckNginxConfIncludeSites(), &result)
	assert.Equal(t, int32(40404), result.Code)
}

func TestCheckNginxConfIncludeStreams(t *testing.T) {
	// test ok
	logger.Init("debug")
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/ok.conf"
	var result *cosy.Error
	errors.As(CheckNginxConfIncludeStreams(), &result)
	assert.Nil(t, result)

	// test 4041 nginx.conf not found
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/4041.conf"
	errors.As(CheckNginxConfIncludeStreams(), &result)
	assert.Equal(t, int32(40402), result.Code)

	// test 5001 nginx.conf parse error
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/5001.conf"
	errors.As(CheckNginxConfIncludeStreams(), &result)
	assert.Equal(t, int32(50001), result.Code)

	// test 4044 nginx.conf no stream block
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/no-http-block.conf"
	errors.As(CheckNginxConfIncludeStreams(), &result)
	assert.Equal(t, int32(40405), result.Code)

	// test 4045 nginx.conf not include stream-enabled
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	settings.NginxSettings.ConfigPath = "./test_cases/no-http-sites-enabled.conf"
	errors.As(CheckNginxConfIncludeStreams(), &result)
	assert.Equal(t, int32(40406), result.Code)
}

func TestFixNginxConfIncludeSites(t *testing.T) {
	logger.Init("debug")
	settings.NginxSettings.ConfigDir = "/etc/nginx"

	// copy file
	content, err := os.ReadFile("./test_cases/no-http-block.conf")
	assert.Nil(t, err)

	err = os.WriteFile("./test_cases/no-http-block-fixed.conf", content, 0644)
	assert.Nil(t, err)

	settings.NginxSettings.ConfigPath = "./test_cases/no-http-block-fixed.conf"
	var result *cosy.Error
	errors.As(FixNginxConfIncludeSites(), &result)
	assert.Nil(t, result)

	// copy file
	content, err = os.ReadFile("./test_cases/no-http-sites-enabled.conf")
	assert.Nil(t, err)
	err = os.WriteFile("./test_cases/no-http-sites-enabled-fixed.conf", content, 0644)
	assert.Nil(t, err)

	settings.NginxSettings.ConfigPath = "./test_cases/no-http-sites-enabled-fixed.conf"
	errors.As(FixNginxConfIncludeSites(), &result)
	assert.Nil(t, result)

	settings.NginxSettings.ConfigPath = "./test_cases/no-http-sites-enabled-fixed.conf"
	errors.As(FixNginxConfIncludeStreams(), &result)
	assert.Nil(t, result)

	// remove backup files (./test_cases/*.bak.*)
	files, err := os.ReadDir("./test_cases")
	assert.Nil(t, err)

	for _, file := range files {
		if strings.Contains(file.Name(), ".bak.") {
			err = os.Remove("./test_cases/" + file.Name())
			assert.Nil(t, err)
		}
	}
}
