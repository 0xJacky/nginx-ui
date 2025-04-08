package site

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Delete deletes a site by removing the file in sites-available
func Delete(name string) (err error) {
	availablePath := nginx.GetConfPath("sites-available", name)

	syncDelete(name)

	s := query.Site
	_, err = s.Where(s.Path.Eq(availablePath)).Unscoped().Delete(&model.Site{})
	if err != nil {
		return
	}

	enabledPath := nginx.GetConfPath("sites-enabled", name)
	maintenancePath := nginx.GetConfPath("sites-available", name+MaintenanceSuffix)

	if !helper.FileExists(availablePath) {
		return ErrSiteNotFound
	}

	if helper.FileExists(enabledPath) {
		return ErrSiteIsEnabled
	}

	if helper.FileExists(maintenancePath) {
		return ErrSiteIsInMaintenance
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
					logger.Error(err)
				}
			}()
			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetHeader("X-Node-Secret", node.Token).
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
