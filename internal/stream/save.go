package stream

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// Save saves a site configuration file
func Save(name string, content string, overwrite bool, syncNodeIds []uint64, postAction string) (err error) {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	if !overwrite && helper.FileExists(path) {
		return ErrDstFileExists
	}

	err = config.ValidateConfigFile(path, content)
	if err != nil {
		return
	}

	err = config.CheckAndCreateHistory(path, content)
	if err != nil {
		return
	}

	// Hold the apply lock for the whole write -> test -> reload sequence so a
	// concurrent configuration or stream mutation cannot make this save fail on
	// somebody else's file. `nginx -t` always covers the whole tree.
	release := config.LockApply()
	defer release()

	snapshot, err := config.CaptureFile(path)
	if err != nil {
		return
	}

	err = config.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return config.RollbackError(err, func() error {
			return snapshot.Restore(path)
		})
	}

	enabledConfigFilePath, err := ResolveEnabledPath(name)
	if err != nil {
		return config.RollbackError(err, func() error {
			return snapshot.Restore(path)
		})
	}

	// A remote namespace is served by its member nodes only, so the local Nginx
	// must neither validate nor load the configuration. Nothing can be enabled
	// locally for such a stream, so the namespace is only resolved when the
	// stream currently participates in the local Nginx.
	remoteDeploy := false
	if helper.FileExists(enabledConfigFilePath) {
		remoteDeploy = IsRemoteDeploy(name)
	}

	if !remoteDeploy && helper.FileExists(enabledConfigFilePath) {
		// Test nginx configuration. A rejected configuration must not survive on
		// disk: the running Nginx keeps its valid in-memory configuration, so the
		// breakage would only surface on the next Nginx start.
		res := nginx.Control(nginx.TestConfig)
		if res.IsError() {
			return config.RollbackError(res.GetError(), func() error {
				return snapshot.Restore(path)
			})
		}

		if postAction == model.PostSyncActionReloadNginx {
			res = nginx.Control(nginx.Reload)
			if res.IsError() {
				return config.RollbackError(res.GetError(), func() error {
					return config.RestoreAndReload(path, snapshot)
				})
			}
		}
	}

	s := query.Stream
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.SyncNodeIDs).
		Updates(&model.Stream{
			SyncNodeIDs: syncNodeIds,
		})
	if err != nil {
		return
	}

	// Moving a stream into a remote namespace detaches it from the local Nginx.
	if remoteDeploy {
		if detachErr := detachFromLocalNginx(name); detachErr != nil {
			logger.Error(detachErr)
		}
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

			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			resp, err := client.R().
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

			// Mirror the deployment intent on the remote node.
			if IsDeployed(name) {
				syncEnable(name)
			}
		}(node)
	}

	wg.Wait()
}
