package site

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

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

	// Test nginx config, if not pass, then disable the site.
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		return rollbackError(res.GetError(), func() error {
			return removeEnabledLink(enabledConfigFilePath)
		})
	}

	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return rollbackError(res.GetError(), func() error {
			return removeEnabledLinkAndReload(enabledConfigFilePath)
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
	logger.Infof("Enabling site on remote node: site=%q node=%q", name, node.Name)
	client := nodeauth.NewRestyClient(node)
	client.SetBaseURL(node.URL)
	resp, err := client.R().
		Post(fmt.Sprintf("/api/sites/%s/enable", name))
	if err != nil {
		logger.Errorf("Remote site enable request failed: site=%q node=%q error=%v", name, node.Name, err)
		notification.Error("Enable Remote Site Error", err.Error(), nil)
		return
	}
	if resp.StatusCode() != http.StatusOK {
		logger.Errorf("Remote site enable rejected: site=%q node=%q status=%d", name, node.Name, resp.StatusCode())
		notification.Error("Enable Remote Site Error", "Enable site %{name} on %{node} failed", NewSyncResult(node.Name, name, resp))
		return
	}
	logger.Infof("Remote site enable succeeded: site=%q node=%q", name, node.Name)
	notification.Success("Enable Remote Site Success", "Enable site %{name} on %{node} successfully", NewSyncResult(node.Name, name, resp))
}
