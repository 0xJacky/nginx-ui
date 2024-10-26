package site

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
	"net/http"
	"os"
	"runtime"
	"sync"
)

// Delete deletes a site by removing the file in sites-available
func Delete(name string) (err error) {
	availablePath := nginx.GetConfPath("sites-available", name)

	s := query.Site
	_, err = s.Where(s.Path.Eq(availablePath)).Unscoped().Delete(&model.Site{})
	if err != nil {
		return
	}

	enabledPath := nginx.GetConfPath("sites-enabled", name)

	if !helper.FileExists(availablePath) {
		return fmt.Errorf("site not found")
	}

	if helper.FileExists(enabledPath) {
		return fmt.Errorf("site is enabled")
	}

	certModel := model.Cert{Filename: name}
	_ = certModel.Remove()

	err = os.Remove(availablePath)
	if err != nil {
		return
	}

	go syncDelete(name)

	return
}

func syncDelete(name string) {
	nodes := getSyncNodes(name)

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Error(err)
				}
			}()
			defer wg.Done()

			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				Delete(fmt.Sprintf("/api/sites/%s", name))
			if err != nil {
				notification.Error("Delete Remote Site Error", err.Error())
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Delete Remote Site Error", string(resp.Body()))
				return
			}
			notification.Success("Delete Remote Site Success", string(resp.Body()))
		}()
	}

	wg.Wait()
}
