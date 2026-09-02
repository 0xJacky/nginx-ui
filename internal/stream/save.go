package stream

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/config"
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

	if !overwrite {
		exists, existsErr := nginx.Exists(path)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			return ErrDstFileExists
		}
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
	isEnabled, err := nginx.Exists(enabledConfigFilePath)
	if err != nil {
		return config.RollbackError(err, func() error {
			return snapshot.Restore(path)
		})
	}
	remoteDeploy := false
	if isEnabled {
		remoteDeploy = IsRemoteDeploy(name)
	}
	reloadRequested := postAction == model.PostSyncActionReloadNginx
	logger.Infof("Nginx apply decision after stream save: stream=%q enabled=%t remote_deploy=%t post_action=%q",
		name, isEnabled, remoteDeploy, postAction)

	if !isEnabled {
		logger.Infof("Skipping Nginx test and reload after stream save: stream=%q reason=not_enabled post_action=%q",
			name, postAction)
	} else if remoteDeploy {
		logger.Infof("Skipping Nginx test and reload after stream save: stream=%q reason=remote_deploy post_action=%q",
			name, postAction)
	} else {
		// Test nginx configuration. A rejected configuration must not survive on
		// disk: the running Nginx keeps its valid in-memory configuration, so the
		// breakage would only surface on the next Nginx start.
		logger.Infof("Testing Nginx configuration after stream save: stream=%q", name)
		res := nginx.Control(nginx.TestConfig)
		if res.IsError() {
			logger.Errorf("Nginx configuration test after stream save failed: stream=%q error=%v", name, res.GetError())
			return config.RollbackError(res.GetError(), func() error {
				return snapshot.Restore(path)
			})
		}
		logger.Infof("Nginx configuration test after stream save succeeded: stream=%q", name)

		if reloadRequested {
			logger.Infof("Reloading Nginx after stream save: stream=%q", name)
			res = nginx.Control(nginx.Reload)
			if res.IsError() {
				logger.Errorf("Nginx reload after stream save failed: stream=%q error=%v", name, res.GetError())
				return config.RollbackError(res.GetError(), func() error {
					return config.RestoreAndReload(path, snapshot)
				})
			}
			logger.Infof("Nginx reload after stream save succeeded: stream=%q", name)
		} else {
			logger.Infof("Skipping Nginx reload after stream save: stream=%q reason=post_action post_action=%q",
				name, postAction)
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

			logger.Infof("Saving stream to remote node: stream=%q node=%q post_action=%q", name, node.Name, postSyncAction)
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
				logger.Errorf("Remote stream save request failed: stream=%q node=%q error=%v", name, node.Name, err)
				notification.Error("Save Remote Stream Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				logger.Errorf("Remote stream save rejected: stream=%q node=%q status=%d", name, node.Name, resp.StatusCode())
				notification.Error("Save Remote Stream Error", "Save stream %{name} to %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			logger.Infof("Remote stream save succeeded: stream=%q node=%q", name, node.Name)
			notification.Success("Save Remote Stream Success", "Save stream %{name} to %{node} successfully", NewSyncResult(node.Name, name, resp))

			// Mirror the deployment intent only on the node that accepted this
			// save. Broadcasting here once per successful node creates N-by-N
			// enable requests and duplicate notifications.
			if IsDeployed(name) {
				syncEnableOnNode(name, node)
			} else {
				logger.Infof("Skipping remote stream enable after save: stream=%q node=%q reason=not_deployed", name, node.Name)
			}
		}(node)
	}

	wg.Wait()
}
