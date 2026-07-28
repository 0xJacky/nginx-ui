package self_check

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

const externalConfigCheckContent = "# Nginx UI external container shared-path check\n"

// CheckExternalContainerConfigShared verifies that local configuration writes
// are visible at the same path inside the configured external Nginx container.
func CheckExternalContainerConfigShared() error {
	if !settings.NginxSettings.RunningInAnotherContainer() {
		return nil
	}

	return checkExternalContainerConfigShared(nginx.GetConfPath(), docker.StatPath)
}

func checkExternalContainerConfigShared(configDir string, statPath func(string) bool) error {
	checkFile, err := os.CreateTemp(configDir, ".nginx-ui-shared-path-check-*")
	if err != nil {
		return cosy.WrapErrorWithParams(ErrExternalConfigCheckFailed, configDir, err.Error())
	}
	checkPath := checkFile.Name()
	defer os.Remove(checkPath)

	if _, err = checkFile.WriteString(externalConfigCheckContent); err != nil {
		checkFile.Close()
		return cosy.WrapErrorWithParams(ErrExternalConfigCheckFailed, configDir, err.Error())
	}
	if err = checkFile.Close(); err != nil {
		return cosy.WrapErrorWithParams(ErrExternalConfigCheckFailed, configDir, err.Error())
	}

	if !statPath(checkPath) {
		return cosy.WrapErrorWithParams(ErrExternalConfigNotShared, configDir)
	}

	return nil
}
