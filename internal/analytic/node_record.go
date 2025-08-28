package analytic

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// nodeCache contains both slice and map for efficient access
type nodeCache struct {
	Nodes   []*model.Node          // For iteration
	NodeMap map[uint64]*model.Node // For fast lookup by ID
}

// NodeRecordManager manages the node status retrieval process
type NodeRecordManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

type RetryConfig struct {
	BaseInterval    time.Duration
	MaxInterval     time.Duration
	MaxRetries      int
	BackoffMultiple float64
}

var defaultRetryConfig = RetryConfig{
	BaseInterval:    5 * time.Second,
	MaxInterval:     30 * time.Second,
	MaxRetries:      10,
	BackoffMultiple: 1.5,
}

type NodeRetryState struct {
	FailureCount int
	LastSuccess  time.Time
	NextRetry    time.Time
}

var (
	retryStates = make(map[uint64]*NodeRetryState)
	retryMutex  sync.Mutex
)

func getRetryState(nodeID uint64) *NodeRetryState {
	retryMutex.Lock()
	defer retryMutex.Unlock()

	if state, exists := retryStates[nodeID]; exists {
		return state
	}

	state := &NodeRetryState{LastSuccess: time.Now(), NextRetry: time.Now()}
	retryStates[nodeID] = state
	return state
}

// updateNodeStatus directly updates node status without condition checks
func updateNodeStatus(nodeID uint64, status bool, reason string) {
	mutex.Lock()
	defer mutex.Unlock()

	now := time.Now()
	if NodeMap[nodeID] == nil {
		NodeMap[nodeID] = &Node{NodeStat: NodeStat{}}
	}
	NodeMap[nodeID].Status = status
	NodeMap[nodeID].ResponseAt = now
}

func calculateNextRetryInterval(failureCount int) time.Duration {
	if failureCount == 0 {
		return defaultRetryConfig.BaseInterval
	}
	interval := defaultRetryConfig.BaseInterval
	for i := 1; i < failureCount; i++ {
		interval = time.Duration(float64(interval) * defaultRetryConfig.BackoffMultiple)
		if interval > defaultRetryConfig.MaxInterval {
			return defaultRetryConfig.MaxInterval
		}
	}
	return interval
}

func shouldRetry(nodeID uint64) bool {
	state := getRetryState(nodeID)
	now := time.Now()

	if state.FailureCount >= defaultRetryConfig.MaxRetries {
		if now.Sub(state.LastSuccess) < 30*time.Second {
			state.FailureCount = 0
			state.NextRetry = now
			return true
		}
		if now.Before(state.NextRetry) {
			return false
		}
		state.FailureCount = defaultRetryConfig.MaxRetries / 2
		state.NextRetry = now
		return true
	}

	return !now.Before(state.NextRetry)
}

func markConnectionFailure(nodeID uint64, err error) {
	state := getRetryState(nodeID)
	state.FailureCount++
	state.NextRetry = time.Now().Add(calculateNextRetryInterval(state.FailureCount))
	updateNodeStatus(nodeID, false, "connection_failed")
}

func markConnectionSuccess(nodeID uint64) {
	state := getRetryState(nodeID)
	state.FailureCount = 0
	state.LastSuccess = time.Now()
	state.NextRetry = time.Now()
	updateNodeStatus(nodeID, true, "connection_success")
}

func logCurrentNodeStatus(prefix string) {
	mutex.Lock()
	defer mutex.Unlock()
	if NodeMap != nil {
		logger.Debugf("%s: NodeMap contains %d nodes", prefix, len(NodeMap))
	}
}

func NewNodeRecordManager(parentCtx context.Context) *NodeRecordManager {
	ctx, cancel := context.WithCancel(parentCtx)
	return &NodeRecordManager{ctx: ctx, cancel: cancel}
}

func (m *NodeRecordManager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		RetrieveNodesStatus(m.ctx)
	}()
}

func (m *NodeRecordManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancel()
	m.wg.Wait()
}

func (m *NodeRecordManager) Restart() {
	m.Stop()
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.Start()
}

var (
	defaultManager *NodeRecordManager
	restartMu      sync.Mutex
)

func InitDefaultManager() {
	if defaultManager != nil {
		defaultManager.Stop()
	}
	defaultManager = NewNodeRecordManager(context.Background())
	defaultManager.Start()
}

func RestartRetrieveNodesStatus() {
	restartMu.Lock()
	defer restartMu.Unlock()
	if defaultManager == nil {
		InitDefaultManager()
	} else {
		defaultManager.Restart()
	}
}

func StartRetrieveNodesStatus(ctx context.Context) *NodeRecordManager {
	manager := NewNodeRecordManager(ctx)
	manager.Start()
	return manager
}

func StartDefaultManager() {
	restartMu.Lock()
	defer restartMu.Unlock()
	if defaultManager != nil {
		defaultManager.Restart()
	} else {
		InitDefaultManager()
	}
}

func cleanupDisabledNodes(enabledEnvIDs []uint64) {
	enabledMap := make(map[uint64]bool)
	for _, id := range enabledEnvIDs {
		enabledMap[id] = true
	}

	retryMutex.Lock()
	for envID := range retryStates {
		if !enabledMap[envID] {
			delete(retryStates, envID)
		}
	}
	retryMutex.Unlock()

	mutex.Lock()
	for envID := range NodeMap {
		if !enabledMap[envID] {
			delete(NodeMap, envID)
		}
	}
	mutex.Unlock()
}

// getEnabledNodes retrieves enabled nodes from cache or database
func getEnabledNodes() ([]*model.Node, error) {
	if cached, found := cache.GetCachedNodes(); found {
		if nc, ok := cached.(*nodeCache); ok {
			return nc.Nodes, nil
		}
	}

	nodeQuery := query.Node
	nodes, err := nodeQuery.Where(nodeQuery.Enabled.Is(true)).Find()
	if err != nil {
		logger.Error("Failed to query enabled nodes:", err)
		return nil, err
	}

	// Create cache with both slice and map
	nodeMap := make(map[uint64]*model.Node, len(nodes))
	for _, node := range nodes {
		nodeMap[node.ID] = node
	}

	nc := &nodeCache{
		Nodes:   nodes,
		NodeMap: nodeMap,
	}

	cache.SetCachedNodes(nc)
	return nodes, nil
}

// isNodeEnabled checks if a node is enabled using cached map for O(1) lookup
func isNodeEnabled(nodeID uint64) bool {
	if cached, found := cache.GetCachedNodes(); found {
		if nc, ok := cached.(*nodeCache); ok {
			_, exists := nc.NodeMap[nodeID]
			return exists
		}
	}

	// Fallback: load cache and check again
	_, err := getEnabledNodes()
	if err != nil {
		return false
	}

	if cached, found := cache.GetCachedNodes(); found {
		if nc, ok := cached.(*nodeCache); ok {
			_, exists := nc.NodeMap[nodeID]
			return exists
		}
	}

	return false
}

func RetrieveNodesStatus(ctx context.Context) {
	logger.Info("RetrieveNodesStatus start")
	defer logger.Info("RetrieveNodesStatus exited")

	mutex.Lock()
	if NodeMap == nil {
		NodeMap = make(TNodeMap)
	}
	mutex.Unlock()

	envCheckTicker := time.NewTicker(30 * time.Second)
	defer envCheckTicker.Stop()
	timeoutCheckTicker := time.NewTicker(10 * time.Second)
	defer timeoutCheckTicker.Stop()

	nodes, err := getEnabledNodes()
	if err != nil {
		logger.Error(err)
		return
	}

	var enabledNodeIDs []uint64
	for _, n := range nodes {
		enabledNodeIDs = append(enabledNodeIDs, n.ID)
	}

	cleanupDisabledNodes(enabledNodeIDs)

	var wg sync.WaitGroup
	defer wg.Wait()

	// Channel to signal when nodes list changes
	nodeUpdateChan := make(chan []uint64, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timeoutCheckTicker.C:
				checkNodeTimeouts(2 * time.Minute)
			case <-envCheckTicker.C:
				currentNodes, err := getEnabledNodes()
				if err != nil {
					logger.Error("Failed to re-query nodes:", err)
					continue
				}
				var currentEnabledIDs []uint64
				for _, n := range currentNodes {
					currentEnabledIDs = append(currentEnabledIDs, n.ID)
				}
				if !equalUint64Slices(enabledNodeIDs, currentEnabledIDs) {
					cleanupDisabledNodes(currentEnabledIDs)
					enabledNodeIDs = currentEnabledIDs
					select {
					case nodeUpdateChan <- currentEnabledIDs:
					default:
					}
				}
			}
		}
	}()

	for _, node := range nodes {
		wg.Add(1)
		go func(n *model.Node) {
			defer wg.Done()
			retryTicker := time.NewTicker(1 * time.Second)
			defer retryTicker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case newEnabledIDs := <-nodeUpdateChan:
					found := false
					for _, id := range newEnabledIDs {
						if id == n.ID {
							found = true
							break
						}
					}
					if !found {
						return
					}
				case <-retryTicker.C:
					if !isNodeEnabled(n.ID) {
						retryMutex.Lock()
						delete(retryStates, n.ID)
						retryMutex.Unlock()
						return
					}
					if !shouldRetry(n.ID) {
						continue
					}
					if err := nodeAnalyticRecord(n, ctx); err != nil {
						if helper.IsUnexpectedWebsocketError(err) {
							logger.Error(err)
						}
						markConnectionFailure(n.ID, err)
					} else {
						markConnectionSuccess(n.ID)
					}
				}
			}
		}(node)
	}

}

func checkNodeTimeouts(timeout time.Duration) {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	for _, node := range NodeMap {
		if node != nil && node.Status && now.Sub(node.ResponseAt) > timeout {
			node.Status = false
			node.ResponseAt = now
		}
	}
}

// equalUint64Slices compares two uint64 slices for equality
func equalUint64Slices(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for comparison
	mapA := make(map[uint64]bool)
	mapB := make(map[uint64]bool)

	for _, v := range a {
		mapA[v] = true
	}
	for _, v := range b {
		mapB[v] = true
	}

	// Compare maps
	for k := range mapA {
		if !mapB[k] {
			return false
		}
	}
	for k := range mapB {
		if !mapA[k] {
			return false
		}
	}

	return true
}

func nodeAnalyticRecord(nodeModel *model.Node, ctx context.Context) error {
	scopeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	node, err := InitNode(nodeModel)
	if err != nil {
		mutex.Lock()
		if NodeMap[nodeModel.ID] == nil {
			NodeMap[nodeModel.ID] = &Node{
				Node:     nodeModel,
				NodeStat: NodeStat{Status: false, ResponseAt: time.Now()},
			}
		} else {
			NodeMap[nodeModel.ID].Status = false
			NodeMap[nodeModel.ID].ResponseAt = time.Now()
		}
		mutex.Unlock()
		return err
	}

	mutex.Lock()
	NodeMap[nodeModel.ID] = node
	mutex.Unlock()

	u, err := nodeModel.GetWebSocketURL("/api/analytic/intro")
	if err != nil {
		return err
	}

	header := http.Header{}
	header.Set("X-Node-Secret", nodeModel.Token)

	dial := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}

	c, _, err := dial.Dial(u, header)
	if err != nil {
		updateNodeStatus(nodeModel.ID, false, "websocket_dial_failed")
		return err
	}

	defer func() {
		c.Close()
		updateNodeStatus(nodeModel.ID, false, "websocket_connection_closed")
	}()

	go func() {
		select {
		case <-scopeCtx.Done():
			_ = c.Close()
		case <-ctx.Done():
			_ = c.Close()
		}
	}()

	for {
		select {
		case <-scopeCtx.Done():
			return ctx.Err()
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var rawMsg json.RawMessage
		err = c.ReadJSON(&rawMsg)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				updateNodeStatus(nodeModel.ID, false, "websocket_error")
				return err
			}
			return nil
		}

		mutex.Lock()
		if NodeMap[nodeModel.ID] == nil {
			NodeMap[nodeModel.ID] = &Node{
				Node:     nodeModel,
				NodeStat: NodeStat{Status: true, ResponseAt: time.Now()},
			}
		} else {
			var fullNode Node
			if err := json.Unmarshal(rawMsg, &fullNode); err == nil && fullNode.Version != "" {
				NodeMap[nodeModel.ID].NodeInfo = fullNode.NodeInfo
				NodeMap[nodeModel.ID].NodeStat = fullNode.NodeStat
			} else {
				var nodeStat NodeStat
				if err := json.Unmarshal(rawMsg, &nodeStat); err == nil {
					NodeMap[nodeModel.ID].NodeStat = nodeStat
				}
			}
			NodeMap[nodeModel.ID].Status = true
			NodeMap[nodeModel.ID].ResponseAt = time.Now()
		}
		mutex.Unlock()
	}
}
