package site

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

// syncDelete dispatches the delete request to the site's sync nodes. It is a
// variable so tests can observe whether a delete reached the remote nodes.
var syncDelete = dispatchSyncDelete

// Delete deletes a site by removing the file in sites-available. Every refusal
// check runs before any side effect, so a refused delete leaves the database
// record, the certificate record, the file and the remote nodes untouched.
func Delete(name string) (err error) {
	availablePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	enabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return err
	}

	maintenancePath, err := ResolveAvailablePath(name + MaintenanceSuffix)
	if err != nil {
		return err
	}

	availableExists, err := nginx.Exists(availablePath)
	if err != nil {
		return err
	}
	if !availableExists {
		return ErrSiteNotFound
	}

	enabledExists, err := nginx.Exists(enabledPath)
	if err != nil {
		return err
	}
	if enabledExists {
		return ErrSiteIsEnabled
	}

	maintenanceExists, err := nginx.Exists(maintenancePath)
	if err != nil {
		return err
	}
	if maintenanceExists {
		return ErrSiteIsInMaintenance
	}

	s := query.Site

	// Remote namespaces keep the enablement flag in the database, so refuse the
	// deletion the same way an enabled local site is refused.
	if IsRemoteDeploy(name) {
		siteModel, err := s.Where(s.Path.Eq(availablePath)).First()
		if err == nil && siteModel.RemoteEnabled {
			return ErrSiteIsEnabled
		}
	}

	// The sync nodes are resolved from the site record, so dispatch before the
	// record is deleted.
	syncDelete(name)

	_, err = s.Where(s.Path.Eq(availablePath)).Unscoped().Delete(&model.Site{})
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
				Delete(fmt.Sprintf("/api/sites/%s", name))
			if err != nil {
				notification.Error("Delete Remote Site Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Delete Remote Site Error", "Delete site %{name} from %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Delete Remote Site Success", "Delete site %{name} from %{node} successfully", NewSyncResult(node.Name, name, resp))
		}()
	}
}
