package site

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
	"net/http"
	"os"
	"runtime"
	"sync"
)

func Rename(oldName string, newName string) (err error) {
	oldPath := nginx.GetConfPath("sites-available", oldName)
	newPath := nginx.GetConfPath("sites-available", newName)

	if oldPath == newPath {
		return
	}

	// check if dst file exists, do not rename
	if helper.FileExists(newPath) {
		return ErrDstFileExists
	}

	s := query.Site
	_, _ = s.Where(s.Path.Eq(oldPath)).Update(s.Path, newPath)

	err = os.Rename(oldPath, newPath)
	if err != nil {
		return
	}

	// recreate a soft link
	oldEnabledConfigFilePath := nginx.GetConfPath("sites-enabled", oldName)
	if helper.SymbolLinkExists(oldEnabledConfigFilePath) {
		_ = os.Remove(oldEnabledConfigFilePath)
		newEnabledConfigFilePath := nginx.GetConfPath("sites-enabled", newName)
		err = os.Symlink(newPath, newEnabledConfigFilePath)
		if err != nil {
			return
		}
	}

	// test nginx configuration
	output := nginx.TestConf()
	if nginx.GetLogLevel(output) > nginx.Warn {
		return fmt.Errorf("%s", output)
	}

	// reload nginx
	output = nginx.Reload()
	if nginx.GetLogLevel(output) > nginx.Warn {
		return fmt.Errorf("%s", output)
	}

	go syncRename(oldName, newName)

	return
}

func syncRename(oldName, newName string) {
	nodes := getSyncNodes(newName)

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
				SetBody(map[string]string{
					"new_name": newName,
				}).
				Post(fmt.Sprintf("/api/sites/%s/rename", oldName))
			if err != nil {
				notification.Error("Rename Remote Site Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Rename Remote Site Error", "Rename site %{name} to %{new_name} on %{node} failed",
					NewSyncResult(node.Name, oldName, resp).
						SetNewName(newName))
				return
			}
			notification.Success("Rename Remote Site Success", "Rename site %{name} to %{new_name} on %{node} successfully",
				NewSyncResult(node.Name, oldName, resp).
					SetNewName(newName))
		}()
	}

	wg.Wait()
}
