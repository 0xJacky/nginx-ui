package stream

import (
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// getSyncNodes returns the nodes that need to be synchronized by site name
func getSyncNodes(name string) (nodes []*model.Environment) {
	configFilePath := nginx.GetConfPath("streams-available", name)
	s := query.Stream
	stream, err := s.Where(s.Path.Eq(configFilePath)).
		Preload(s.EnvGroup).First()
	if err != nil {
		logger.Error(err)
		return
	}

	syncNodeIds := stream.SyncNodeIDs
	// inherit sync node ids from site category
	if stream.EnvGroup != nil {
		syncNodeIds = append(syncNodeIds, stream.EnvGroup.SyncNodeIds...)
	}

	e := query.Environment
	nodes, err = e.Where(e.ID.In(syncNodeIds...)).Find()
	if err != nil {
		logger.Error(err)
		return
	}
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
