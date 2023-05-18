package analytic

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/gorilla/websocket"
	"github.com/opentracing/opentracing-go/log"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/net"
	"math"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Node struct {
	EnvironmentID int                `json:"environment_id,omitempty"`
	Name          string             `json:"name,omitempty"`
	AvgLoad       *load.AvgStat      `json:"avg_load"`
	CPUPercent    float64            `json:"cpu_percent"`
	MemoryPercent float64            `json:"memory_percent"`
	DiskPercent   float64            `json:"disk_percent"`
	Network       net.IOCountersStat `json:"network"`
	Status        bool               `json:"status"`
}

var mutex sync.Mutex

type TNodeMap map[int]*Node

var NodeMap TNodeMap

func init() {
	NodeMap = make(TNodeMap)
}

func nodeAnalyticLive(env *model.Environment, errChan chan error) {
	for {
		err := nodeAnalyticRecord(env)

		if err != nil {
			// set node offline
			if NodeMap[env.ID] != nil {
				NodeMap[env.ID].Status = false
			}
			log.Error(err)
			errChan <- err
			// wait 5s then reconnect
			time.Sleep(5 * time.Second)
		}
	}
}

func nodeAnalyticRecord(env *model.Environment) (err error) {
	url, err := env.GetWebSocketURL("/api/analytic/intro")

	if err != nil {
		return
	}

	header := http.Header{}

	header.Set("X-Node-Secret", env.Token)

	c, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		return
	}

	defer c.Close()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}
		logger.Debugf("recv: %s %s", env.Name, message)

		var nodeAnalytic Node

		err = json.Unmarshal(message, &nodeAnalytic)

		if err != nil {
			return err
		}

		nodeAnalytic.EnvironmentID = env.ID
		nodeAnalytic.Name = env.Name
		// set online
		nodeAnalytic.Status = true
		mutex.Lock()
		NodeMap[env.ID] = &nodeAnalytic
		mutex.Unlock()
	}
}

func RetrieveNodesStatus() {
	NodeMap = make(TNodeMap)

	env := query.Environment

	envs, err := env.Find()

	if err != nil {
		logger.Error(err)
		return
	}

	errChan := make(chan error)

	for _, v := range envs {
		go nodeAnalyticLive(v, errChan)
	}

	// block at here
	for err = range errChan {
		log.Error(err)
	}
}

func GetNodeAnalyticIntro() (data Node) {
	memory, err := GetMemoryStat()

	if err != nil {
		logger.Error(err)
		return
	}

	cpuTimesBefore, _ := cpu.Times(false)
	time.Sleep(1000 * time.Millisecond)
	cpuTimesAfter, _ := cpu.Times(false)
	threadNum := runtime.GOMAXPROCS(0)
	cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
	cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)

	loadAvg, err := load.Avg()

	if err != nil {
		logger.Error(err)
		return
	}

	diskStat, err := GetDiskStat()

	if err != nil {
		logger.Error(err)
		return
	}

	netIO, err := net.IOCounters(false)

	if err != nil {
		logger.Error(err)
		return
	}

	var network net.IOCountersStat
	if len(netIO) > 0 {
		network = netIO[0]
	}

	return Node{
		AvgLoad:       loadAvg,
		CPUPercent:    math.Min((cpuUserUsage+cpuSystemUsage)*100, 100),
		MemoryPercent: memory.Pressure,
		DiskPercent:   diskStat.Percentage,
		Network:       network,
	}
}
