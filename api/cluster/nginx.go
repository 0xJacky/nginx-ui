package cluster

import (
	"net/http"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

type syncResult struct {
	Node string `json:"node"`
	Resp string `json:"resp"`
}

// syncReload handle reload nginx on remote nodes
func syncReload(nodeIDs []uint64) {
	if len(nodeIDs) == 0 {
		return
	}

	e := query.Environment
	nodes, err := e.Where(e.ID.In(nodeIDs...)).Find()
	if err != nil {
		logger.Error("Failed to get environment nodes:", err)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(node *model.Environment) {
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
				Post("/api/nginx/reload")
			if err != nil {
				notification.Error("Reload Remote Nginx Error", "", err.Error())
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Reload Remote Nginx Error",
					"Reload Nginx on %{node} failed, response: %{resp}", syncResult{
						Node: node.Name,
						Resp: resp.String(),
					})
				return
			}
			notification.Success("Reload Remote Nginx Success",
				"Reload Nginx on %{node} successfully", syncResult{
					Node: node.Name,
				})
		}(node)
	}

	wg.Wait()
}

// syncRestart handle restart nginx on remote nodes
func syncRestart(nodeIDs []uint64) {
	if len(nodeIDs) == 0 {
		return
	}

	e := query.Environment
	nodes, err := e.Where(e.ID.In(nodeIDs...)).Find()
	if err != nil {
		logger.Error("Failed to get environment nodes:", err)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(node *model.Environment) {
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
				Post("/api/nginx/restart")
			if err != nil {
				notification.Error("Restart Remote Nginx Error", "", err.Error())
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Restart Remote Nginx Error",
					"Restart Nginx on %{node} failed, response: %{resp}", syncResult{
						Node: node.Name,
						Resp: resp.String(),
					})
				return
			}
			notification.Success("Restart Remote Nginx Success",
				"Restart Nginx on %{node} successfully", syncResult{
					Node: node.Name,
				})
		}(node)
	}

	wg.Wait()
}
