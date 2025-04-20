package stream

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func Rename(oldName string, newName string) (err error) {
	oldPath := nginx.GetConfPath("streams-available", oldName)
	newPath := nginx.GetConfPath("streams-available", newName)

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
	oldEnabledConfigFilePath := nginx.GetConfPath("streams-enabled", oldName)
	if helper.SymbolLinkExists(oldEnabledConfigFilePath) {
		_ = os.Remove(oldEnabledConfigFilePath)
		newEnabledConfigFilePath := nginx.GetConfPath("streams-enabled", newName)
		err = os.Symlink(newPath, newEnabledConfigFilePath)
		if err != nil {
			return
		}
	}

	// test nginx configuration
	output, err := nginx.TestConfig()
	if err != nil {
		return
	}
	if nginx.GetLogLevel(output) > nginx.Warn {
		return cosy.WrapErrorWithParams(ErrNginxTestFailed, output)
	}

	// reload nginx
	output, err = nginx.Reload()
	if err != nil {
		return
	}
	if nginx.GetLogLevel(output) > nginx.Warn {
		return cosy.WrapErrorWithParams(ErrNginxReloadFailed, output)
	}

	// update ChatGPT history
	g := query.ChatGPTLog
	_, _ = g.Where(g.Name.Eq(oldName)).Update(g.Name, newName)

	// update config history
	b := query.ConfigBackup
	_, _ = b.Where(b.FilePath.Eq(oldPath)).Updates(map[string]interface{}{
		"filepath": newPath,
		"name":     newName,
	})

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
				Post(fmt.Sprintf("/api/streams/%s/rename", oldName))
			if err != nil {
				notification.Error("Rename Remote Stream Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Rename Remote Stream Error", "Rename stream %{name} to %{new_name} on %{node} failed",
					NewSyncResult(node.Name, oldName, resp).
						SetNewName(newName))
				return
			}
			notification.Success("Rename Remote Stream Success", "Rename stream %{name} to %{new_name} on %{node} successfully",
				NewSyncResult(node.Name, oldName, resp).
					SetNewName(newName))
		}()
	}

	wg.Wait()
}
