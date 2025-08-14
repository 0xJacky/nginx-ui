package site

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Save saves a site configuration file
func Save(name string, content string, overwrite bool, namespaceId uint64, syncNodeIds []uint64, postAction string) (err error) {
	path := nginx.GetConfPath("sites-available", name)
	if !overwrite && helper.FileExists(path) {
		return ErrDstFileExists
	}

	err = config.CheckAndCreateHistory(path, content)
	if err != nil {
		return
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return
	}

	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)
	if helper.FileExists(enabledConfigFilePath) {
		// Test nginx configuration
		c := nginx.Control(nginx.TestConfig)
		if c.IsError() {
			return c.GetError()
		}

		if postAction == model.PostSyncActionReloadNginx {
			c := nginx.Control(nginx.Reload)
			if c.IsError() {
				return c.GetError()
			}
		}
	}

	s := query.Site
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.NamespaceID, s.SyncNodeIDs).
		Updates(&model.Site{
			NamespaceID: namespaceId,
			SyncNodeIDs: syncNodeIds,
		})
	if err != nil {
		return
	}

	go syncSave(name, content)

	return
}

func syncSave(name string, content string) {
	nodes, postSyncAction := getSyncData(name)

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	// Map to track successful nodes for potential post-sync action
	successfulNodes := make([]*model.Node, 0)
	var nodesMutex sync.Mutex

	for _, node := range nodes {
		go func(node *model.Node) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			defer wg.Done()

			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetHeader("X-Node-Secret", node.Token).
				SetBody(map[string]interface{}{
					"content":     content,
					"overwrite":   true,
					"post_action": postSyncAction,
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

			// Track successful sync for post-sync action
			nodesMutex.Lock()
			successfulNodes = append(successfulNodes, node)
			nodesMutex.Unlock()

			// Check if the site is enabled, if so then enable it on the remote node
			enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)
			if helper.FileExists(enabledConfigFilePath) {
				syncEnable(name)
			}
		}(node)
	}

	wg.Wait()
}
