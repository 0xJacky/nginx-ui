package site

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// Disable disables a site by removing the symlink in sites-enabled
func Disable(name string) (err error) {
	enabledConfigFilePath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		return err
	}

	// Remote namespaces keep their deployment intent in the database because no
	// local symlink is ever created for them.
	if IsRemoteDeploy(name) {
		if err = setRemoteEnabled(name, false); err != nil {
			return
		}

		go syncDisable(name)

		return
	}

	// Already disabled: keep the operation idempotent so cluster syncs can
	// converge a node without reporting spurious failures.
	if !helper.FileExists(enabledConfigFilePath) {
		return
	}

	err = os.Remove(enabledConfigFilePath)
	if err != nil {
		return
	}

	// delete auto cert record
	certModel := model.Cert{Filename: name}
	err = certModel.Remove()
	if err != nil {
		return
	}

	res := nginx.Control(nginx.Reload)
	if res.IsError() {
		return res.GetError()
	}

	go syncDisable(name)

	return
}

func syncDisable(name string) {
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
				Post(fmt.Sprintf("/api/sites/%s/disable", name))
			if err != nil {
				notification.Error("Disable Remote Site Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Disable Remote Site Error", "Disable site %{name} from %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Disable Remote Site Success", "Disable site %{name} from %{node} successfully", NewSyncResult(node.Name, name, resp))
		}()
	}

	wg.Wait()
}
