package site

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
	"net/http"
	"os"
	"runtime"
	"sync"
)

// Disable disables a site by removing the symlink in sites-enabled
func Disable(name string) (err error) {
	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)
	_, err = os.Stat(enabledConfigFilePath)
	if err != nil {
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

	output := nginx.Reload()
	if nginx.GetLogLevel(output) > nginx.Warn {
		return fmt.Errorf(output)
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
					logger.Error(err)
				}
			}()
			defer wg.Done()

			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetHeader("X-Node-Secret", node.Token).
				Post(fmt.Sprintf("/api/sites/%s/disable", name))
			if err != nil {
				notification.Error("Disable Remote Site Error", err.Error())
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Disable Remote Site Error", NewSyncResult(node.Name, name, resp).String())
				return
			}
			notification.Success("Disable Remote Site Success", NewSyncResult(node.Name, name, resp).String())
		}()
	}

	wg.Wait()
}
