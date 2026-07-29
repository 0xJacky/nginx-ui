package stream

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
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
		return res.GetError()
	}

	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return res.GetError()
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
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			defer wg.Done()

			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				Post(fmt.Sprintf("/api/streams/%s/enable", name))
			if err != nil {
				notification.Error("Enable Remote Stream Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Enable Remote Stream Error", "Enable stream %{name} on %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Enable Remote Stream Success", "Enable stream %{name} on %{node} successfully", NewSyncResult(node.Name, name, resp))
		}()
	}

	wg.Wait()
}
