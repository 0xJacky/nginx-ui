package self_check

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

const externalConfigCheckContent = "# Nginx UI external container shared-path check\n"

// needsSharedConfigCheck reports whether the configuration directory must be
// visible on the nginx target at the same path. That holds for an external
// container and for an SSH host with a mounted directory. SFTP access has no
// bind mount by design: the probe would be written into this container and
// looked up on the host, so it would always report an unshared directory.
func needsSharedConfigCheck(n *settings.Nginx) bool {
	return n.ControlMode() != settings.ControlModeLocal && !n.UsesSFTP()
}

// CheckExternalContainerConfigShared verifies that local configuration writes
// are visible at the same path inside the configured external Nginx container.
func CheckExternalContainerConfigShared() error {
	if !needsSharedConfigCheck(settings.NginxSettings) {
		return nil
	}

	// StatOnTarget routes to docker exec or SSH depending on the control mode,
	// so the same evidence works for both.
	return checkExternalContainerConfigShared(nginx.GetConfPath(), nginx.StatOnTarget)
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
