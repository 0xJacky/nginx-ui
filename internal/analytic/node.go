package analytic

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

type NodeInfo struct {
	NodeRuntimeInfo version.RuntimeInfo `json:"node_runtime_info"`
	Version         string              `json:"version"`
	CPUNum          int                 `json:"cpu_num"`
	MemoryTotal     string              `json:"memory_total"`
	DiskTotal       string              `json:"disk_total"`
}

type NodeStat struct {
	AvgLoad           *load.AvgStat               `json:"avg_load"`
	CPUPercent        float64                     `json:"cpu_percent"`
	MemoryPercent     float64                     `json:"memory_percent"`
	DiskPercent       float64                     `json:"disk_percent"`
	Network           net.IOCountersStat          `json:"network"`
	Status            bool                        `json:"status"`
	ResponseAt        time.Time                   `json:"response_at"`
	UpstreamStatusMap map[string]*upstream.Status `json:"upstream_status_map"`
}

type Node struct {
	NodeID int `json:"node_id,omitempty"`
	*model.Node
	NodeStat
	NodeInfo
}

var mutex sync.Mutex

type TNodeMap map[uint64]*Node

var NodeMap TNodeMap

func init() {
	NodeMap = make(TNodeMap)
}

func GetNode(node *model.Node) (n *Node) {
	if node == nil {
		// this should never happen
		logger.Error("node is nil")
		return
	}
	if !node.Enabled {
		return &Node{
			Node: node,
		}
	}
	n, ok := NodeMap[node.ID]
	if !ok {
		n = &Node{}
	}
	n.Node = node
	return n
}

func InitNode(node *model.Node) (n *Node, err error) {
	n = &Node{
		Node: node,
	}

	u, err := url.JoinPath(node.URL, "/api/node")
	if err != nil {
		return
	}

	t, err := transport.NewTransport()
	if err != nil {
		return
	}
	client := http.Client{
		Transport: t,
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}

	req.Header.Set("X-Node-Secret", node.Token)

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	bytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return n, cosy.WrapErrorWithParams(ErrNodeAnalyticsFailed, string(bytes))
	}

	err = json.Unmarshal(bytes, &n.NodeInfo)
	if err != nil {
		return
	}

	return
}
