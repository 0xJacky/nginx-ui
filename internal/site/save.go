package site

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
func Save(name string, content string, overwrite bool, namespaceId uint64, syncNodeIds []uint64, postAction string) (err error) {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}
	enabledConfigFilePath, err := ResolveEnabledPath(name)
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

	// Hold the apply lock for the whole write -> test -> reload sequence so a
	// concurrent configuration or site mutation cannot make this save fail on
	// somebody else's file. `nginx -t` always covers the whole tree.
	release := config.LockApply()
	defer release()

	snapshot, err := captureConfigFile(path)
	if err != nil {
		return err
	}

	err = config.ValidateConfigFile(path, content)
	if err != nil {
		return
	}

	err = config.CheckAndCreateHistory(path, content)
	if err != nil {
		return
	}

	err = writeConfigFile(path, []byte(content), 0644)
	if err != nil {
		return rollbackError(err, func() error {
			return snapshot.Restore(path)
		})
	}

	// A remote namespace is served by its member nodes only, so the local Nginx
	// must neither validate nor load the configuration. Nothing can be enabled
	// locally for such a site, so the namespace is only resolved when the site
	// currently participates in the local Nginx.
	isEnabled, err := nginx.Exists(enabledConfigFilePath)
	if err != nil {
		return rollbackError(err, func() error {
			return snapshot.Restore(path)
		})
	}
	remoteDeploy := false
	if isEnabled {
		remoteDeploy = ResolveNamespaceByID(namespaceId).IsRemoteDeploy()
	}
	reloadRequested := postAction == model.PostSyncActionReloadNginx
	logger.Infof("Nginx apply decision after site save: site=%q enabled=%t remote_deploy=%t post_action=%q",
		name, isEnabled, remoteDeploy, postAction)

	if !isEnabled {
		logger.Infof("Skipping Nginx test and reload after site save: site=%q reason=not_enabled post_action=%q",
			name, postAction)
	} else if remoteDeploy {
		logger.Infof("Skipping Nginx test and reload after site save: site=%q reason=remote_deploy post_action=%q",
			name, postAction)
	} else {
		// Test nginx configuration
		logger.Infof("Testing Nginx configuration after site save: site=%q", name)
		c := nginx.Control(nginx.TestConfig)
		if c.IsError() {
			logger.Errorf("Nginx configuration test after site save failed: site=%q error=%v", name, c.GetError())
			return rollbackError(c.GetError(), func() error {
				return snapshot.Restore(path)
			})
		}
		logger.Infof("Nginx configuration test after site save succeeded: site=%q", name)

		if reloadRequested {
			logger.Infof("Reloading Nginx after site save: site=%q", name)
			c := nginx.Control(nginx.Reload)
			if c.IsError() {
				logger.Errorf("Nginx reload after site save failed: site=%q error=%v", name, c.GetError())
				return rollbackError(c.GetError(), func() error {
					return restoreConfigAndReload(path, snapshot)
				})
			}
			logger.Infof("Nginx reload after site save succeeded: site=%q", name)
		} else {
			logger.Infof("Skipping Nginx reload after site save: site=%q reason=post_action post_action=%q",
				name, postAction)
		}
	}

	s := query.Site
	// The record has to exist before the namespace and the sync targets can be
	// stored on it, otherwise a freshly created site never joins its namespace.
	if namespaceId > 0 || len(syncNodeIds) > 0 {
		if _, err = s.Where(s.Path.Eq(path)).FirstOrCreate(); err != nil {
			return rollbackError(err, func() error {
				return snapshot.Restore(path)
			})
		}
	}

	_, err = s.Where(s.Path.Eq(path)).
		Select(s.NamespaceID, s.SyncNodeIDs).
		Updates(&model.Site{
			NamespaceID: namespaceId,
			SyncNodeIDs: syncNodeIds,
		})
	if err != nil {
		return rollbackError(err, func() error {
			if !remoteDeploy && isEnabled && postAction == model.PostSyncActionReloadNginx {
				return restoreConfigAndReload(path, snapshot)
			}
			return snapshot.Restore(path)
		})
	}

	// Moving a site into a remote namespace detaches it from the local Nginx and
	// carries the previous enablement over to the remote deployment intent.
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

			logger.Infof("Saving site to remote node: site=%q node=%q post_action=%q", name, node.Name, postSyncAction)
			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetBody(map[string]interface{}{
					"content":     content,
					"overwrite":   true,
					"post_action": postSyncAction,
				}).
				Post(fmt.Sprintf("/api/sites/%s", name))
			if err != nil {
				logger.Errorf("Remote site save request failed: site=%q node=%q error=%v", name, node.Name, err)
				notification.Error("Save Remote Site Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				logger.Errorf("Remote site save rejected: site=%q node=%q status=%d", name, node.Name, resp.StatusCode())
				notification.Error("Save Remote Site Error", "Save site %{name} to %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			logger.Infof("Remote site save succeeded: site=%q node=%q", name, node.Name)
			notification.Success("Save Remote Site Success", "Save site %{name} to %{node} successfully", NewSyncResult(node.Name, name, resp))

			// Mirror the deployment intent only on the node that accepted this
			// save. Broadcasting here once per successful node creates N-by-N
			// enable requests and duplicate notifications.
			if IsDeployed(name) {
				syncEnableOnNode(name, node)
			} else {
				logger.Infof("Skipping remote site enable after save: site=%q node=%q reason=not_deployed", name, node.Name)
			}
		}(node)
	}

	wg.Wait()
}
