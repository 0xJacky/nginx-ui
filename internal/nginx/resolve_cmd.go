package nginx

import (
	"os/exec"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

type nginxStringCache struct {
	mutex sync.Mutex
	value string
}

func (c *nginxStringCache) get(loader func() string) string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.value == "" {
		c.value = loader()
	}
	return c.value
}

func (c *nginxStringCache) set(value string) {
	c.mutex.Lock()
	c.value = value
	c.mutex.Unlock()
}

var (
	nginxSbinPathCache nginxStringCache
	nginxVOutputCache  nginxStringCache
	nginxTOutputCache  nginxStringCache
)

// resetPathCaches drops everything derived from the target nginx. The values
// describe one control target, so switching targets must not keep serving the
// previous one.
func resetPathCaches() {
	nginxSbinPathCache.set("")
	nginxVOutputCache.set("")
	nginxTOutputCache.set("")
	nginxPrefixCache.set("")
}

// Returns the path to the nginx executable
func getNginxSbinPath() string {
	// The configured path is read on every call so a settings change takes
	// effect without a restart. Only the discovered path is cached.
	if settings.NginxSettings.SbinPath != "" {
		return settings.NginxSettings.SbinPath
	}

	return nginxSbinPathCache.get(func() string {
		var path string
		var err error
		if runtime.GOOS == "windows" {
			path, err = exec.LookPath("nginx.exe")
		} else {
			path, err = exec.LookPath("nginx")
		}
		if err == nil {
			return path
		}
		return ""
	})
}

func getNginxV() string {
	return nginxVOutputCache.get(func() string {
		exePath := getNginxSbinPath()
		out, err := execCommand(exePath, "-V")
		if err != nil {
			logger.Error(err)
			return ""
		}
		return out
	})
}

// getNginxT executes nginx -T and returns the output
func getNginxT() string {
	return nginxTOutputCache.get(func() string {
		exePath := getNginxSbinPath()
		out, err := execCommand(exePath, "-T")
		if err != nil {
			logger.Error(err)
			return ""
		}
		return out
	})
}
