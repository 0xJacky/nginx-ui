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
	"github.com/uozi-tech/cosy/logger"
)

func Rename(oldName string, newName string) (err error) {
	oldPath, err := ResolveAvailablePath(oldName)
	if err != nil {
		return err
	}

	newPath, err := ResolveAvailablePath(newName)
	if err != nil {
		return err
	}

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
	oldEnabledConfigFilePath, err := ResolveEnabledPath(oldName)
	if err != nil {
		return err
	}

	if helper.SymbolLinkExists(oldEnabledConfigFilePath) {
		_ = os.Remove(oldEnabledConfigFilePath)
		var newEnabledConfigFilePath string
		newEnabledConfigFilePath, err = ResolveEnabledPath(newName)
		if err != nil {
			return err
		}

		err = os.Symlink(newPath, newEnabledConfigFilePath)
		if err != nil {
			return
		}
	}

	// test nginx configuration
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		return res.GetError()
	}

	// reload nginx
	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return res.GetError()
	}

	// update LLM history
	g := query.LLMSession
	_, _ = g.Where(g.Path.Eq(oldPath)).Update(g.Path, newPath)

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
					logger.Errorf("%s\n%s", err, buf)
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
