package site

import (
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

// getSyncData returns the nodes that need to be synchronized by site name and the post-sync action
func getSyncData(name string) (nodes []*model.Node, postSyncAction string) {
	configFilePath := nginx.GetConfPath("sites-available", name)
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
func getSyncNodes(name string) (nodes []*model.Node) {
	nodes, _ = getSyncData(name)
	return
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
