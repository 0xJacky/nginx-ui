package convert

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// pendingSyncs tracks the background replications started by Convert so
// callers that swap global settings (tests) can wait for them first.
var pendingSyncs sync.WaitGroup

// WaitForSync blocks until every background replication started by Convert
// has returned.
func WaitForSync() {
	pendingSyncs.Wait()
}

// SyncResult is the notification payload of a remote conversion.
type SyncResult struct {
	StatusCode int    `json:"status_code"`
	Node       string `json:"node"`
	Site       string `json:"site"`
	Upstream   string `json:"upstream"`
	Response   gin.H  `json:"response"`
}

func newSyncResult(node string, req Request, resp *resty.Response) *SyncResult {
	result := &SyncResult{
		StatusCode: resp.StatusCode(),
		Node:       node,
		Site:       req.Site,
		Upstream:   req.Upstream,
	}
	if err := json.Unmarshal(resp.Body(), &result.Response); err != nil {
		logger.Error(err)
	}
	return result
}

// siteSyncNodes returns the nodes the site is replicated to. A site without a
// database record has none.
func siteSyncNodes(p *plan) []*model.Node {
	s := query.Site
	found, err := s.Where(s.Path.Eq(p.sitePath)).Limit(1).Find()
	if err != nil || len(found) == 0 {
		return nil
	}
	return site.GetSyncNodes(p.siteName)
}

// startSync mirrors the conversion. Nodes that receive the site run the same
// conversion themselves: pushing the two files one after the other would fail
// on the node, because in between the upstream is either defined twice or not
// at all. Nodes that only receive conf.d get the new group file, as a
// managed upstream save would send it.
//
// A conversion another node replicated here is not mirrored again: that node
// already reached every target, and mirroring it back would loop.
func startSync(ctx context.Context, p *plan, cfg *model.Config, name, userName string) {
	if nodeauth.IsReplicated(ctx) {
		logger.Infof("Skipping upstream conversion sync for a replicated change: site=%q upstream=%q", p.siteName, name)
		return
	}

	nodes := siteSyncNodes(p)
	nodeIDs := make([]uint64, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.ID)
	}

	if err := config.SyncToRemoteServerExcept(ctx, cfg, userName, nodeIDs); err != nil {
		logger.Error(err)
	}

	req := Request{Site: p.siteName, Upstream: name}
	for _, node := range nodes {
		pendingSyncs.Add(1)
		go func(node *model.Node) {
			defer pendingSyncs.Done()
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			syncConvert(node, req)
		}(node)
	}
}

func syncConvert(node *model.Node, req Request) {
	client := nodeauth.NewRestyClient(node)
	client.SetBaseURL(node.URL)
	resp, err := client.R().SetBody(req).Post("/api/upstream/convert")
	if err != nil {
		logger.Errorf("Remote upstream conversion request failed: site=%q upstream=%q node=%q error=%v",
			req.Site, req.Upstream, node.Name, err)
		notification.Error("Convert Remote Upstream Error", err.Error(), nil)
		return
	}
	result := newSyncResult(node.Name, req, resp)
	if resp.StatusCode() != http.StatusOK {
		logger.Errorf("Remote upstream conversion rejected: site=%q upstream=%q node=%q status=%d",
			req.Site, req.Upstream, node.Name, resp.StatusCode())
		notification.Error("Convert Remote Upstream Error",
			"Convert upstream %{upstream} of site %{site} on %{node} failed", result)
		return
	}
	notification.Success("Convert Remote Upstream Success",
		"Convert upstream %{upstream} of site %{site} on %{node} successfully", result)
}
