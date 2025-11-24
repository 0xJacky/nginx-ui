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
	*model.Node
	NodeStat
	NodeInfo
}

var nodeMapMu sync.RWMutex

type TNodeMap map[uint64]*Node

var NodeMap TNodeMap

func init() {
	NodeMap = make(TNodeMap)
}

func cloneNode(n *Node) *Node {
	if n == nil {
		return nil
	}

	cloned := *n

	if n.Node != nil {
		nodeCopy := *n.Node
		cloned.Node = &nodeCopy
	}

	if n.UpstreamStatusMap != nil {
		upstreams := make(map[string]*upstream.Status, len(n.UpstreamStatusMap))
		for key, status := range n.UpstreamStatusMap {
			if status == nil {
				upstreams[key] = nil
				continue
			}
			statusCopy := *status
			upstreams[key] = &statusCopy
		}
		cloned.UpstreamStatusMap = upstreams
	}

	return &cloned
}

func SnapshotNodeMap() TNodeMap {
	nodeMapMu.RLock()
	defer nodeMapMu.RUnlock()

	snapshot := make(TNodeMap, len(NodeMap))
	for id, node := range NodeMap {
		snapshot[id] = cloneNode(node)
	}

	return snapshot
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
	nodeMapMu.RLock()
	cached, ok := NodeMap[node.ID]
	nodeMapMu.RUnlock()
	if !ok || cached == nil {
		return &Node{
			Node: node,
		}
	}

	cloned := cloneNode(cached)
	if cloned == nil {
		return &Node{
			Node: node,
		}
	}
	cloned.Node = node
	return cloned
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
