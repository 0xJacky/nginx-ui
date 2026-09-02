package analytic

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// nodeCache avoids querying the enabled-node list for every worker retry.
type nodeCache struct {
	Nodes []*model.Node
}

type RetryConfig struct {
	BaseInterval    time.Duration
	MaxInterval     time.Duration
	BackoffMultiple float64
}

var defaultRetryConfig = RetryConfig{
	BaseInterval:    5 * time.Second,
	MaxInterval:     30 * time.Second,
	BackoffMultiple: 1.5,
}

type NodeRetryState struct {
	FailureCount int
	NextRetry    time.Time
}

var (
	retryStates = make(map[uint64]*NodeRetryState)
	retryMutex  sync.Mutex
	nodeReload  = make(chan struct{}, 1)
)

const nodeOfflineTimeout = 2 * time.Minute

// WebSocket keepalive timings for the connection to remote nodes.
// pongWait bounds how long ReadJSON may block; pingPeriod must be < pongWait so
// the peer has a chance to respond before the deadline fires. Declared as var
// (not const) so tests can shorten them without redefining the production
// defaults.
var (
	nodeWSWriteWait  = 10 * time.Second
	nodeWSPongWait   = 60 * time.Second
	nodeWSPingPeriod = (nodeWSPongWait * 9) / 10
)

func markNodeOfflineIfStale(nodeID uint64, timeout time.Duration) {
	nodeMapMu.Lock()
	defer nodeMapMu.Unlock()

	node := NodeMap[nodeID]
	if node == nil || !node.Status {
		return
	}
	if node.ResponseAt.IsZero() || time.Since(node.ResponseAt) >= timeout {
		node.Status = false
	}
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
	retryMutex.Lock()
	defer retryMutex.Unlock()

	state, exists := retryStates[nodeID]
	if !exists {
		state = &NodeRetryState{NextRetry: time.Now()}
		retryStates[nodeID] = state
	}
	now := time.Now()
	return !now.Before(state.NextRetry)
}

func markConnectionFailure(nodeID uint64, connectionErr error) int {
	retryMutex.Lock()
	state, exists := retryStates[nodeID]
	if !exists {
		state = &NodeRetryState{}
		retryStates[nodeID] = state
	}
	state.FailureCount++
	failureCount := state.FailureCount
	state.NextRetry = time.Now().Add(calculateNextRetryInterval(state.FailureCount))
	retryMutex.Unlock()

	nodeMapMu.Lock()
	if node := NodeMap[nodeID]; node != nil {
		failedAt := time.Now()
		node.ConnectionError = connectionErr.Error()
		node.ConnectionErrorCode = classifyNodeConnectionError(connectionErr, failedAt)
		node.ConnectionErrorAt = &failedAt
	}
	nodeMapMu.Unlock()

	markNodeOfflineIfStale(nodeID, nodeOfflineTimeout)
	return failureCount
}

func markConnectionSuccess(nodeID uint64) bool {
	retryMutex.Lock()
	state, exists := retryStates[nodeID]
	if !exists {
		state = &NodeRetryState{}
		retryStates[nodeID] = state
	}
	recovered := state.FailureCount > 0
	state.FailureCount = 0
	state.NextRetry = time.Now()
	retryMutex.Unlock()

	nodeMapMu.Lock()
	if node := NodeMap[nodeID]; node != nil {
		node.ConnectionError = ""
		node.ConnectionErrorCode = ""
		node.ConnectionErrorAt = nil
	}
	nodeMapMu.Unlock()
	return recovered
}

func classifyNodeConnectionError(connectionErr error, now time.Time) NodeConnectionErrorCode {
	if connectionErr == nil {
		return ""
	}

	var certificateInvalidError x509.CertificateInvalidError
	if errors.As(connectionErr, &certificateInvalidError) &&
		certificateInvalidError.Cert != nil &&
		now.Before(certificateInvalidError.Cert.NotBefore) {
		return NodeConnectionErrorClockSkew
	}

	message := connectionErr.Error()
	if strings.Contains(message, "current time") &&
		strings.Contains(message, "is before") &&
		strings.Contains(message, "not yet valid") {
		return NodeConnectionErrorClockSkew
	}
	if strings.Contains(message, "node signature creation time is in the future") ||
		strings.Contains(message, "node signature is expired") {
		return NodeConnectionErrorClockSkew
	}

	return ""
}

// ReloadNodesStatus asks the single monitor loop started by the kernel to
// rebuild its workers. It deliberately does not start another monitor: doing
// so lets two connections race to publish status for the same node.
func ReloadNodesStatus() {
	select {
	case nodeReload <- struct{}{}:
	default:
	}
}

func cleanupDisabledNodes(enabledNodeIDs []uint64) {
	enabledMap := make(map[uint64]bool)
	for _, id := range enabledNodeIDs {
		enabledMap[id] = true
	}

	retryMutex.Lock()
	for nodeID := range retryStates {
		if !enabledMap[nodeID] {
			delete(retryStates, nodeID)
		}
	}
	retryMutex.Unlock()

	nodeMapMu.Lock()
	for nodeID := range NodeMap {
		if !enabledMap[nodeID] {
			delete(NodeMap, nodeID)
		}
	}
	nodeMapMu.Unlock()
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

	nc := &nodeCache{
		Nodes: nodes,
	}

	cache.SetCachedNodes(nc)
	return nodes, nil
}

func RetrieveNodesStatus(ctx context.Context) {
	logger.Info("RetrieveNodesStatus start")
	defer logger.Info("RetrieveNodesStatus exited")

	nodeMapMu.Lock()
	if NodeMap == nil {
		NodeMap = make(TNodeMap)
	}
	nodeMapMu.Unlock()

	for ctx.Err() == nil {
		reload, err := runNodeStatusCycle(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.Error("Failed to start node status workers:", err)
			select {
			case <-ctx.Done():
				return
			case <-nodeReload:
			case <-time.After(defaultRetryConfig.BaseInterval):
			}
			continue
		}
		if !reload {
			return
		}
	}
}

// runNodeStatusCycle owns exactly one worker per enabled node. A configuration
// change cancels and joins the whole cycle before fresh workers are created, so
// an old connection can never overwrite the status published by its replacement.
func runNodeStatusCycle(ctx context.Context) (reload bool, err error) {
	nodes, err := getEnabledNodes()
	if err != nil {
		return false, err
	}

	enabledNodeIDs := make([]uint64, 0, len(nodes))
	nodeMapMu.Lock()
	for _, node := range nodes {
		enabledNodeIDs = append(enabledNodeIDs, node.ID)
		if existing := NodeMap[node.ID]; existing == nil {
			NodeMap[node.ID] = &Node{Node: node}
		} else {
			existing.Node = node
		}
	}
	nodeMapMu.Unlock()
	cleanupDisabledNodes(enabledNodeIDs)
	retryMutex.Lock()
	for _, nodeID := range enabledNodeIDs {
		delete(retryStates, nodeID)
	}
	retryMutex.Unlock()

	cycleCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(n *model.Node) {
			defer wg.Done()
			runNodeStatusWorker(cycleCtx, n)
		}(node)
	}

	nodeCheckTicker := time.NewTicker(30 * time.Second)
	timeoutCheckTicker := time.NewTicker(10 * time.Second)
	defer func() {
		nodeCheckTicker.Stop()
		timeoutCheckTicker.Stop()
		cancel()
		wg.Wait()
	}()
	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-nodeReload:
			return true, nil
		case <-timeoutCheckTicker.C:
			checkNodeTimeouts(nodeOfflineTimeout)
		case <-nodeCheckTicker.C:
			currentNodes, queryErr := getEnabledNodes()
			if queryErr != nil {
				logger.Error("Failed to re-query nodes:", queryErr)
				continue
			}
			if !equalNodeConfigs(nodes, currentNodes) {
				return true, nil
			}
		}
	}
}

func runNodeStatusWorker(ctx context.Context, node *model.Node) {
	retryTicker := time.NewTicker(time.Second)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-retryTicker.C:
			if !shouldRetry(node.ID) {
				continue
			}
			if err := nodeAnalyticRecord(node, ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				failureCount := markConnectionFailure(node.ID, err)
				if failureCount == 1 {
					logger.Warnf("Node status connection failed for node %d (%q): %v", node.ID, node.Name, err)
				}
			}
		}
	}
}

func checkNodeTimeouts(timeout time.Duration) {
	nodeMapMu.Lock()
	defer nodeMapMu.Unlock()
	now := time.Now()
	for _, node := range NodeMap {
		if node != nil && node.Status && now.Sub(node.ResponseAt) > timeout {
			node.Status = false
		}
	}
}

func equalNodeConfigs(a, b []*model.Node) bool {
	if len(a) != len(b) {
		return false
	}

	nodesByID := make(map[uint64]*model.Node, len(a))
	for _, node := range a {
		nodesByID[node.ID] = node
	}

	for _, node := range b {
		previous, exists := nodesByID[node.ID]
		if !exists {
			return false
		}
		if previous.Name != node.Name || previous.URL != node.URL ||
			previous.AuthMethod != node.AuthMethod ||
			!bytes.Equal(previous.EncryptedLegacySecret, node.EncryptedLegacySecret) ||
			previous.Enabled != node.Enabled {
			return false
		}
	}

	return true
}

func nodeAnalyticRecord(nodeModel *model.Node, ctx context.Context) error {
	scopeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Snapshot keepalive timings once so test overrides cannot race with the
	// background ping loop after this function has started.
	pongWait := nodeWSPongWait
	pingPeriod := nodeWSPingPeriod
	writeWait := nodeWSWriteWait

	node, err := InitNode(scopeCtx, nodeModel)
	if err != nil {
		nodeMapMu.Lock()
		if NodeMap[nodeModel.ID] == nil {
			NodeMap[nodeModel.ID] = &Node{
				Node: nodeModel,
			}
		} else {
			NodeMap[nodeModel.ID].Node = nodeModel
		}
		nodeMapMu.Unlock()
		return fmt.Errorf("node HTTP probe failed: %w", err)
	}

	nodeMapMu.Lock()
	if existing := NodeMap[nodeModel.ID]; existing == nil {
		NodeMap[nodeModel.ID] = node
	} else {
		existing.Node = nodeModel
		existing.NodeInfo = node.NodeInfo
	}
	nodeMapMu.Unlock()

	u, err := nodeModel.GetWebSocketURL("/api/analytic/intro")
	if err != nil {
		return fmt.Errorf("build node WebSocket URL: %w", err)
	}

	header := http.Header{}
	if err := nodeauth.SignWebSocketHeaders(nodeModel, u, header); err != nil {
		return fmt.Errorf("sign node WebSocket request: %w", err)
	}

	dial := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}

	c, _, err := dial.DialContext(scopeCtx, u, header)
	if err != nil {
		return fmt.Errorf("connect node WebSocket: %w", err)
	}

	defer func() {
		_ = c.Close()
	}()

	// Arm read deadline and refresh it on every pong. Without this, a silently
	// half-dead TCP connection (NAT drop, peer hang) would block ReadJSON below
	// indefinitely, freezing this node's retry loop until the process restarts.
	_ = c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(pongWait))
	})

	go func() {
		select {
		case <-scopeCtx.Done():
			_ = c.Close()
		case <-ctx.Done():
			_ = c.Close()
		}
	}()

	// Periodic ping keeps the connection warm and triggers the deadline above
	// when the peer stops responding.
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-scopeCtx.Done():
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
					_ = c.Close()
					return
				}
			}
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
			return fmt.Errorf("read node WebSocket status: %w", err)
		}

		nodeMapMu.Lock()
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
		nodeMapMu.Unlock()
		if markConnectionSuccess(nodeModel.ID) {
			logger.Infof("Node status connection restored for node %d (%q)", nodeModel.ID, nodeModel.Name)
		}
	}
}
