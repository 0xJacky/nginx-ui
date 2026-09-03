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
	"github.com/uozi-tech/cosy/logger"
)

// Enable enables a site by creating a symlink in sites-enabled
func Enable(name string) (err error) {
	configFilePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	enabledConfigFilePath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		return err
	}

	_, err = nginx.Stat(configFilePath)
	if err != nil {
		return
	}

	// Remote namespaces are served by their member nodes: record the intent and
	// dispatch it instead of touching the local Nginx.
	if IsRemoteDeploy(name) {
		if err = setRemoteEnabled(name, true); err != nil {
			return
		}

		go syncEnable(name)

		return
	}

	enabledExists, err := nginx.Exists(enabledConfigFilePath)
	if err != nil {
		return err
	}
	if enabledExists {
		return
	}

	err = nginx.Symlink(configFilePath, enabledConfigFilePath)
	if err != nil {
		return
	}

	// Test nginx config, if not pass, then disable the stream. Leaving the fresh
	// symlink behind would keep the broken configuration on disk and break the
	// next Nginx start.
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		return config.RollbackError(res.GetError(), func() error {
			return config.RemoveEnabledLink(enabledConfigFilePath)
		})
	}

	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return config.RollbackError(res.GetError(), func() error {
			return config.RemoveEnabledLinkAndReload(enabledConfigFilePath)
		})
	}

	go syncEnable(name)

	return
}

func syncEnable(name string) {
	nodes := getSyncNodes(name)

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

			syncEnableOnNode(name, node)
		}(node)
	}

	wg.Wait()
}

// syncEnableOnNode mirrors the deployment intent to one node. Save calls this
// after that same node accepts the configuration, avoiding an N-by-N broadcast.
func syncEnableOnNode(name string, node *model.Node) {
	logger.Infof("Enabling stream on remote node: stream=%q node=%q", name, node.Name)
	client := nodeauth.NewRestyClient(node)
	client.SetBaseURL(node.URL)
	resp, err := client.R().
		Post(fmt.Sprintf("/api/streams/%s/enable", name))
	if err != nil {
		logger.Errorf("Remote stream enable request failed: stream=%q node=%q error=%v", name, node.Name, err)
		notification.Error("Enable Remote Stream Error", err.Error(), nil)
		return
	}
	if resp.StatusCode() != http.StatusOK {
		logger.Errorf("Remote stream enable rejected: stream=%q node=%q status=%d", name, node.Name, resp.StatusCode())
		notification.Error("Enable Remote Stream Error", "Enable stream %{name} on %{node} failed", NewSyncResult(node.Name, name, resp))
		return
	}
	logger.Infof("Remote stream enable succeeded: stream=%q node=%q", name, node.Name)
	notification.Success("Enable Remote Stream Success", "Enable stream %{name} on %{node} successfully", NewSyncResult(node.Name, name, resp))
}
