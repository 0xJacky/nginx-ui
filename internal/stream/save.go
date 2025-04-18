package stream

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
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// Save saves a site configuration file
func Save(name string, content string, overwrite bool, syncNodeIds []uint64, postAction string) (err error) {
	path := nginx.GetConfPath("streams-available", name)
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

	enabledConfigFilePath := nginx.GetConfPath("streams-enabled", name)
	if helper.FileExists(enabledConfigFilePath) {
		// Test nginx configuration
		output := nginx.TestConf()

		if nginx.GetLogLevel(output) > nginx.Warn {
			return cosy.WrapErrorWithParams(ErrNginxTestFailed, output)
		}

		if postAction == model.PostSyncActionReloadNginx {
			output = nginx.Reload()
			if nginx.GetLogLevel(output) > nginx.Warn {
				return cosy.WrapErrorWithParams(ErrNginxReloadFailed, output)
			}
		}
	}

	s := query.Stream
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.SyncNodeIDs).
		Updates(&model.Site{
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
	successfulNodes := make([]*model.Environment, 0)
	var nodesMutex sync.Mutex

	for _, node := range nodes {
		go func(node *model.Environment) {
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
					"content":     content,
					"overwrite":   true,
					"post_action": postSyncAction,
				}).
				Post(fmt.Sprintf("/api/streams/%s", name))
			if err != nil {
				notification.Error("Save Remote Stream Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Save Remote Stream Error", "Save stream %{name} to %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Save Remote Stream Success", "Save stream %{name} to %{node} successfully", NewSyncResult(node.Name, name, resp))

			// Track successful sync for post-sync action
			nodesMutex.Lock()
			successfulNodes = append(successfulNodes, node)
			nodesMutex.Unlock()

			// Check if the site is enabled, if so then enable it on the remote node
			enabledConfigFilePath := nginx.GetConfPath("streams-enabled", name)
			if helper.FileExists(enabledConfigFilePath) {
				syncEnable(name)
			}
		}(node)
	}

	wg.Wait()
}
