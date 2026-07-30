package stream

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// Delete deletes a site by removing the file in sites-available
func Delete(name string) (err error) {
	availablePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	s := query.Stream

	// Remote namespaces keep the enablement flag in the database, so refuse the
	// deletion the same way an enabled local stream is refused.
	remoteDeploy := IsRemoteDeploy(name)
	if remoteDeploy {
		streamModel, err := s.Where(s.Path.Eq(availablePath)).First()
		if err == nil && streamModel.RemoteEnabled {
			return ErrStreamIsEnabled
		}
	}

	syncDelete(name)

	_, err = s.Where(s.Path.Eq(availablePath)).Unscoped().Delete(&model.Stream{})
	if err != nil {
		return
	}

	enabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return err
	}

	if !helper.FileExists(availablePath) {
		return ErrStreamNotFound
	}

	if !remoteDeploy && helper.FileExists(enabledPath) {
		return ErrStreamIsEnabled
	}

	certModel := model.Cert{Filename: name}
	_ = certModel.Remove()

	err = os.Remove(availablePath)
	if err != nil {
		return
	}

	return
}

func syncDelete(name string) {
	nodes := getSyncNodes(name)

	for _, node := range nodes {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				Delete(fmt.Sprintf("/api/streams/%s", name))
			if err != nil {
				notification.Error("Delete Remote Stream Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Delete Remote Stream Error", "Delete stream %{name} from %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Delete Remote Stream Success", "Delete stream %{name} from %{node} successfully", NewSyncResult(node.Name, name, resp))
		}()
	}
}
