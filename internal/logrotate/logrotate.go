package logrotate

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"os/exec"
	"strings"
)

func Exec() {
	if !settings.LogrotateSettings.Enabled {
		return
	}

	logger.Info("logrotate start")
	defer logger.Info("logrotate end")
	cmd := strings.Split(settings.LogrotateSettings.CMD, " ")

	if len(cmd) == 0 {
		return
	}

	var (
		name string
		args = make([]string, 0)
	)

	if len(cmd) > 0 {
		name = cmd[0]
	}

	if len(cmd) > 1 {
		args = cmd[1:]
	}

	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		logger.Error(err, string(out))
		return
	}

	if len(out) > 0 {
		logger.Debug(string(out))
	}
}
