package analytic

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/internal/upgrader"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/uozi-tech/cosy/logger"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type NodeInfo struct {
	NodeRuntimeInfo upgrader.RuntimeInfo `json:"node_runtime_info"`
	Version         string               `json:"version"`
	CPUNum          int                  `json:"cpu_num"`
	MemoryTotal     string               `json:"memory_total"`
	DiskTotal       string               `json:"disk_total"`
}

type NodeStat struct {
	AvgLoad       *load.AvgStat      `json:"avg_load"`
	CPUPercent    float64            `json:"cpu_percent"`
	MemoryPercent float64            `json:"memory_percent"`
	DiskPercent   float64            `json:"disk_percent"`
	Network       net.IOCountersStat `json:"network"`
	Status        bool               `json:"status"`
	ResponseAt    time.Time          `json:"response_at"`
}

type Node struct {
	EnvironmentID int `json:"environment_id,omitempty"`
	*model.Environment
	NodeStat
	NodeInfo
}

var mutex sync.Mutex

type TNodeMap map[uint64]*Node

var NodeMap TNodeMap

func init() {
	NodeMap = make(TNodeMap)
}

func GetNode(env *model.Environment) (n *Node) {
	if env == nil {
		// this should never happen
		logger.Error("env is nil")
		return
	}
	if !env.Enabled {
		return &Node{
			Environment: env,
		}
	}
	n, ok := NodeMap[env.ID]
	if !ok {
		n = &Node{}
	}
	n.Environment = env
	return n
}

func InitNode(env *model.Environment) (n *Node) {
	n = &Node{
		Environment: env,
	}

	u, err := url.JoinPath(env.URL, "/api/node")
	if err != nil {
		logger.Error(err)
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
		logger.Error(err)
		return
	}

	req.Header.Set("X-Node-Secret", env.Token)

	resp, err := client.Do(req)
	if err != nil {
		logger.Error(err)
		return
	}

	defer resp.Body.Close()
	bytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		logger.Error(string(bytes))
		return
	}

	err = json.Unmarshal(bytes, &n.NodeInfo)
	if err != nil {
		logger.Error(err)
		return
	}

	return
}
