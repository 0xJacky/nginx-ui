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
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Save saves a site configuration file
func Save(name string, content string, overwrite bool, envGroupId uint64, syncNodeIds []uint64) (err error) {
	path := nginx.GetConfPath("sites-available", name)
	if !overwrite && helper.FileExists(path) {
		return ErrDstFileExists
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return
	}

	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)
	if helper.FileExists(enabledConfigFilePath) {
		// Test nginx configuration
		output := nginx.TestConf()

		if nginx.GetLogLevel(output) > nginx.Warn {
			return fmt.Errorf("%s", output)
		}

		output = nginx.Reload()

		if nginx.GetLogLevel(output) > nginx.Warn {
			return fmt.Errorf("%s", output)
		}
	}

	s := query.Site
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.EnvGroupID, s.SyncNodeIDs).
		Updates(&model.Site{
			EnvGroupID:  envGroupId,
			SyncNodeIDs: syncNodeIds,
		})
	if err != nil {
		return
	}

	go syncSave(name, content)

	return
}

func syncSave(name string, content string) {
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
				SetBody(map[string]interface{}{
					"content":   content,
					"overwrite": true,
				}).
				Post(fmt.Sprintf("/api/sites/%s", name))
			if err != nil {
				notification.Error("Save Remote Site Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Save Remote Site Error", "Save site %{name} to %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Save Remote Site Success", "Save site %{name} to %{node} successfully", NewSyncResult(node.Name, name, resp))

			// Check if the site is enabled, if so then enable it on the remote node
			enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)
			if helper.FileExists(enabledConfigFilePath) {
				syncEnable(name)
			}
		}()
	}

	wg.Wait()
}
