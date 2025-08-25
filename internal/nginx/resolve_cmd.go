package nginx

import (
	"os/exec"
	"runtime"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

var (
	nginxSbinPath string
	nginxVOutput  string
	nginxTOutput  string
)

// Returns the path to the nginx executable
func getNginxSbinPath() string {
	// load from cache
	if nginxSbinPath != "" {
		return nginxSbinPath
	}

	// load from settings
	if settings.NginxSettings.SbinPath != "" {
		nginxSbinPath = settings.NginxSettings.SbinPath
		return nginxSbinPath
	}

	// load from system
	var path string
	var err error
	if runtime.GOOS == "windows" {
		path, err = exec.LookPath("nginx.exe")
	} else {
		path, err = exec.LookPath("nginx")
	}
	if err == nil {
		nginxSbinPath = path
		return nginxSbinPath
	}
	return nginxSbinPath
}

func getNginxV() string {
	// load from cache
	if nginxVOutput != "" {
		return nginxVOutput
	}

	// load from system
	exePath := getNginxSbinPath()
	out, err := execCommand(exePath, "-V")
	if err != nil {
		logger.Error(err)
		return ""
	}

	nginxVOutput = out
	return nginxVOutput
}

// getNginxT executes nginx -T and returns the output
func getNginxT() string {
	// load from cache
	if nginxTOutput != "" {
		return nginxTOutput
	}

	// load from system
	exePath := getNginxSbinPath()
	out, err := execCommand(exePath, "-T")
	if err != nil {
		logger.Error(err)
		return ""
	}

	nginxTOutput = out
	return nginxTOutput
}
