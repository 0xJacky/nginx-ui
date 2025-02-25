package site

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Enable enables a site by creating a symlink in sites-enabled
func Enable(name string) (err error) {
	configFilePath := nginx.GetConfPath("sites-available", name)
	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)

	_, err = os.Stat(configFilePath)
	if err != nil {
		return
	}

	if helper.FileExists(enabledConfigFilePath) {
		return
	}

	err = os.Symlink(configFilePath, enabledConfigFilePath)
	if err != nil {
		return
	}

	// Test nginx config, if not pass, then disable the site.
	output := nginx.TestConf()
	if nginx.GetLogLevel(output) > nginx.Warn {
		_ = os.Remove(enabledConfigFilePath)
		return fmt.Errorf(output)
	}

	output = nginx.Reload()
	if nginx.GetLogLevel(output) > nginx.Warn {
		return fmt.Errorf(output)
	}

	go syncEnable(name)

	return
}

func syncEnable(name string) {
	nodes := getSyncNodes(name)

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Error(err)
				}
			}()
			defer wg.Done()

			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetHeader("X-Node-Secret", node.Token).
				Post(fmt.Sprintf("/api/sites/%s/enable", name))
			if err != nil {
				notification.Error("Enable Remote Site Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Enable Remote Site Error", "Enable site %{name} on %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Enable Remote Site Success", "Enable site %{name} on %{node} successfully", NewSyncResult(node.Name, name, resp))
		}()
	}

	wg.Wait()
}
