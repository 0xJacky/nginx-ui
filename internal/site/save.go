package site

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
	"net/http"
	"os"
	"runtime"
	"sync"
)

// Save saves a site configuration file
func Save(name string, content string, overwrite bool, siteCategoryId uint64, syncNodeIds []uint64) (err error) {
	path := nginx.GetConfPath("sites-available", name)
	if !overwrite && helper.FileExists(path) {
		return fmt.Errorf("file exists")
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
			return fmt.Errorf(output)
		}

		output = nginx.Reload()

		if nginx.GetLogLevel(output) > nginx.Warn {
			return fmt.Errorf(output)
		}
	}

	s := query.Site
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.SiteCategoryID, s.SyncNodeIDs).
		Updates(&model.Site{
			SiteCategoryID: siteCategoryId,
			SyncNodeIDs:    syncNodeIds,
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
				SetBody(map[string]interface{}{
					"content":   content,
					"overwrite": true,
				}).
				Post(fmt.Sprintf("/api/sites/%s", name))
			if err != nil {
				notification.Error("Save Remote Site Error", err.Error())
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Save Remote Site Error", string(resp.Body()))
				return
			}
			notification.Success("Save Remote Site Success", string(resp.Body()))
		}()
	}

	wg.Wait()
}
