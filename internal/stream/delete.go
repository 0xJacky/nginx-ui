package stream

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// syncDelete dispatches the delete request to the stream's sync nodes. It is a
// variable so tests can observe whether a delete reached the remote nodes.
var syncDelete = dispatchSyncDelete

// Delete deletes a stream by removing the file in streams-available. Every
// refusal check runs before any side effect, so a refused delete leaves the
// database record, the file and the remote nodes untouched.
func Delete(name string) (err error) {
	availablePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	enabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return err
	}

	availableExists, err := nginx.Exists(availablePath)
	if err != nil {
		return err
	}
	if !availableExists {
		return ErrStreamNotFound
	}

	s := query.Stream

	// Remote namespaces keep the enablement flag in the database instead of a
	// streams-enabled symlink, so refuse the deletion the same way an enabled
	// local stream is refused.
	if IsRemoteDeploy(name) {
		streamModel, err := s.Where(s.Path.Eq(availablePath)).First()
		if err == nil && streamModel.RemoteEnabled {
			return ErrStreamIsEnabled
		}
	} else {
		enabledExists, err := nginx.Exists(enabledPath)
		if err != nil {
			return err
		}
		if enabledExists {
			return ErrStreamIsEnabled
		}
	}

	// The sync nodes are resolved from the stream record, so dispatch before the
	// record is deleted.
	syncDelete(name)

	_, err = s.Where(s.Path.Eq(availablePath)).Unscoped().Delete(&model.Stream{})
	if err != nil {
		return
	}

	certModel := model.Cert{Filename: name}
	_ = certModel.Remove()

	return nginx.Remove(availablePath)
}

func dispatchSyncDelete(name string) {
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
