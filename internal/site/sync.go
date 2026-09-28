package site

import (
	"context"
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

// getSyncData returns the nodes and namespace metadata that need to be synchronized by site name.
// A change that another node replicated here has no targets: the node where
// it was made already fanned it out, and forwarding it again would loop.
func getSyncData(ctx context.Context, name string) (nodes []*model.Node, postSyncAction, namespaceName string) {
	if nodeauth.IsReplicated(ctx) {
		logger.Infof("Skipping site sync for a replicated change: site=%q", name)
		return
	}

	configFilePath, err := ResolveAvailablePath(name)
	if err != nil {
		logger.Error(err)
		return
	}

	s := query.Site
	site, err := s.Where(s.Path.Eq(configFilePath)).
		Preload(s.Namespace).First()
	if err != nil {
		logger.Error(err)
		return
	}

	syncNodeIds := site.SyncNodeIDs
	// inherit sync node ids from site category
	if site.Namespace != nil {
		syncNodeIds = append(syncNodeIds, site.Namespace.SyncNodeIds...)
		postSyncAction = site.Namespace.PostSyncAction
		namespaceName = site.Namespace.Name
	}
	syncNodeIds = lo.Uniq(syncNodeIds)

	n := query.Node
	nodes, err = n.Where(n.ID.In(syncNodeIds...)).Find()
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

// getSyncNodes returns the nodes that need to be synchronized by site name (for backward compatibility)
func getSyncNodes(ctx context.Context, name string) (nodes []*model.Node) {
	nodes, _, _ = getSyncData(ctx, name)
	return
}

// GetSyncNodes returns the nodes configured for a site and its namespace.
// Callers use this to synchronize metadata that belongs to the logical site
// but is stored outside the Nginx configuration file.
func GetSyncNodes(name string) []*model.Node {
	return getSyncNodes(context.Background(), name)
}

type SyncResult struct {
	StatusCode int    `json:"status_code"`
	Node       string `json:"node"`
	Name       string `json:"name"`
	NewName    string `json:"new_name,omitempty"`
	Response   gin.H  `json:"response"`
	Error      string `json:"error"`
}

func NewSyncResult(node string, siteName string, resp *resty.Response) (s *SyncResult) {
	s = &SyncResult{
		StatusCode: resp.StatusCode(),
		Node:       node,
		Name:       siteName,
	}
	err := json.Unmarshal(resp.Body(), &s.Response)
	if err != nil {
		logger.Error(err)
	}
	return
}

func (s *SyncResult) SetNewName(name string) *SyncResult {
	s.NewName = name
	return s
}
