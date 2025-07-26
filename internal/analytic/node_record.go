package analytic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// NodeRecordManager manages the node status retrieval process
type NodeRecordManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	BaseInterval    time.Duration // Base retry interval
	MaxInterval     time.Duration // Maximum retry interval
	MaxRetries      int           // Maximum consecutive failures before giving up temporarily
	BackoffMultiple float64       // Multiplier for exponential backoff
	ResetAfter      time.Duration // Time to reset failure count if successful
}

// Default retry configuration
var defaultRetryConfig = RetryConfig{
	BaseInterval:    5 * time.Second,  // Start with 5 seconds
	MaxInterval:     5 * time.Minute,  // Max 5 minutes between retries
	MaxRetries:      10,               // Max 10 consecutive failures
	BackoffMultiple: 1.5,              // 1.5x backoff each time
	ResetAfter:      30 * time.Second, // Reset failure count after 30s of success
}

// NodeRetryState tracks retry state for each node
type NodeRetryState struct {
	FailureCount    int
	LastRetryTime   time.Time
	LastSuccessTime time.Time
	NextRetryTime   time.Time
}

var (
	retryStates = make(map[uint64]*NodeRetryState)
	retryMutex  sync.Mutex
)

// getRetryState gets or creates retry state for a node
func getRetryState(envID uint64) *NodeRetryState {
	retryMutex.Lock()
	defer retryMutex.Unlock()

	if state, exists := retryStates[envID]; exists {
		return state
	}

	state := &NodeRetryState{
		FailureCount:    0,
		LastSuccessTime: time.Now(),
		NextRetryTime:   time.Now(),
	}
	retryStates[envID] = state
	return state
}

// updateNodeStatus safely updates node status with proper timestamp
func updateNodeStatus(envID uint64, status bool, reason string) {
	mutex.Lock()
	defer mutex.Unlock()

	now := time.Now()
	if NodeMap[envID] != nil {
		NodeMap[envID].Status = status
		NodeMap[envID].ResponseAt = now
		logger.Debugf("updateNodeStatus: Node[%d] status updated to %t (%s) at %v",
			envID, status, reason, now)
	} else {
		logger.Debugf("updateNodeStatus: Warning - Node[%d] not found in NodeMap", envID)
	}
}

// calculateNextRetryInterval calculates the next retry interval using exponential backoff
func calculateNextRetryInterval(state *NodeRetryState, config RetryConfig) time.Duration {
	if state.FailureCount == 0 {
		return config.BaseInterval
	}

	interval := config.BaseInterval
	for i := 0; i < state.FailureCount-1; i++ {
		interval = time.Duration(float64(interval) * config.BackoffMultiple)
		if interval > config.MaxInterval {
			interval = config.MaxInterval
			break
		}
	}

	logger.Debugf("calculateNextRetryInterval: FailureCount=%d, NextInterval=%v",
		state.FailureCount, interval)
	return interval
}

// shouldRetry determines if we should retry connection for a node
func shouldRetry(envID uint64, config RetryConfig) bool {
	state := getRetryState(envID)
	now := time.Now()

	// Check if we've exceeded max retries
	if state.FailureCount >= config.MaxRetries {
		// If we've been successful recently, reset the failure count
		if now.Sub(state.LastSuccessTime) < config.ResetAfter {
			logger.Debugf("shouldRetry: Resetting failure count for node %d due to recent success", envID)
			state.FailureCount = 0
			state.NextRetryTime = now
			return true
		}

		// Too many failures, back off for a longer period
		if now.Before(state.NextRetryTime) {
			logger.Debugf("shouldRetry: Node %d in backoff period until %v (failures: %d)",
				envID, state.NextRetryTime, state.FailureCount)
			return false
		}

		// Reset after long backoff period
		logger.Debugf("shouldRetry: Resetting failure count for node %d after backoff period", envID)
		state.FailureCount = config.MaxRetries / 2 // Start from middle to avoid immediate max again
		state.NextRetryTime = now
		return true
	}

	// Normal retry logic
	if now.Before(state.NextRetryTime) {
		return false
	}

	return true
}

// markConnectionFailure marks a connection failure and calculates next retry time
func markConnectionFailure(envID uint64, config RetryConfig, err error) {
	state := getRetryState(envID)
	now := time.Now()

	state.FailureCount++
	state.LastRetryTime = now

	nextInterval := calculateNextRetryInterval(state, config)
	state.NextRetryTime = now.Add(nextInterval)

	logger.Debugf("markConnectionFailure: Node %d failed (count: %d), next retry at %v, error: %v",
		envID, state.FailureCount, state.NextRetryTime, err)

	// Update node status to offline
	updateNodeStatus(envID, false, "connection_failed")
}

// markConnectionSuccess marks a successful connection
func markConnectionSuccess(envID uint64) {
	state := getRetryState(envID)
	now := time.Now()

	state.FailureCount = 0
	state.LastSuccessTime = now
	state.NextRetryTime = now // Can retry immediately if connection drops

	logger.Debugf("markConnectionSuccess: Node %d connection successful, failure count reset", envID)

	// Status will be updated in nodeAnalyticRecord when we receive actual data
}

// logCurrentNodeStatus logs current node status for debugging
func logCurrentNodeStatus(prefix string) {
	mutex.Lock()
	defer mutex.Unlock()

	if NodeMap == nil {
		logger.Debugf("%s: NodeMap is nil", prefix)
		return
	}

	logger.Debugf("%s: Current NodeMap contains %d nodes", prefix, len(NodeMap))
	for envID, node := range NodeMap {
		if node == nil {
			logger.Debugf("%s: Node[%d] is nil", prefix, envID)
			continue
		}

		// Also log retry state
		retryMutex.Lock()
		state := retryStates[envID]
		retryMutex.Unlock()

		retryInfo := "no_retry_state"
		if state != nil {
			retryInfo = fmt.Sprintf("failures=%d,next_retry=%v",
				state.FailureCount, state.NextRetryTime)
		}

		logger.Debugf("%s: Node[%d] - Status: %t, ResponseAt: %v, RetryState: %s",
			prefix, envID, node.Status, node.ResponseAt, retryInfo)
	}
}

// NewNodeRecordManager creates a new NodeRecordManager with the provided context
func NewNodeRecordManager(parentCtx context.Context) *NodeRecordManager {
	logger.Debug("Creating new NodeRecordManager")
	ctx, cancel := context.WithCancel(parentCtx)
	return &NodeRecordManager{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins retrieving node status using the manager's context
func (m *NodeRecordManager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Debug("NodeRecordManager: Starting node status retrieval")
	logCurrentNodeStatus("NodeRecordManager.Start - Before start")

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		RetrieveNodesStatus(m.ctx)
	}()

	logger.Debug("NodeRecordManager: Started successfully")
}

// Stop cancels the current context and waits for operations to complete
func (m *NodeRecordManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Debug("NodeRecordManager: Stopping node status retrieval")
	logCurrentNodeStatus("NodeRecordManager.Stop - Before stop")

	m.cancel()
	m.wg.Wait()

	logger.Debug("NodeRecordManager: Stopped successfully")
	logCurrentNodeStatus("NodeRecordManager.Stop - After stop")
}

// Restart stops and then restarts the node status retrieval
func (m *NodeRecordManager) Restart() {
	logger.Debug("NodeRecordManager: Restarting node status retrieval")
	logCurrentNodeStatus("NodeRecordManager.Restart - Before restart")

	m.Stop()

	logger.Debug("NodeRecordManager: Creating new context for restart")
	// Create new context
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// Start retrieval with new context
	m.Start()

	logger.Debug("NodeRecordManager: Restart completed")
	logCurrentNodeStatus("NodeRecordManager.Restart - After restart")
}

// For backward compatibility
var (
	defaultManager *NodeRecordManager
	restartMu      sync.Mutex
)

// InitDefaultManager initializes the default NodeRecordManager
func InitDefaultManager() {
	logger.Debug("Initializing default NodeRecordManager")
	logCurrentNodeStatus("InitDefaultManager - Before init")

	if defaultManager != nil {
		logger.Debug("Default manager exists, stopping it first")
		defaultManager.Stop()
	}
	defaultManager = NewNodeRecordManager(context.Background())
	defaultManager.Start()

	logger.Debug("Default NodeRecordManager initialized")
	logCurrentNodeStatus("InitDefaultManager - After init")
}

// RestartRetrieveNodesStatus restarts the node status retrieval process
// Kept for backward compatibility
func RestartRetrieveNodesStatus() {
	restartMu.Lock()
	defer restartMu.Unlock()

	logger.Debug("RestartRetrieveNodesStatus called")
	logCurrentNodeStatus("RestartRetrieveNodesStatus - Before restart")

	if defaultManager == nil {
		logger.Debug("Default manager is nil, initializing new one")
		InitDefaultManager()
		return
	}

	logger.Debug("Restarting existing default manager")
	defaultManager.Restart()

	logger.Debug("RestartRetrieveNodesStatus completed")
	logCurrentNodeStatus("RestartRetrieveNodesStatus - After restart")
}

// StartRetrieveNodesStatus starts the node status retrieval with a custom context
func StartRetrieveNodesStatus(ctx context.Context) *NodeRecordManager {
	logger.Debug("Starting node status retrieval with custom context")
	logCurrentNodeStatus("StartRetrieveNodesStatus - Before start")

	manager := NewNodeRecordManager(ctx)
	manager.Start()

	logger.Debug("Custom NodeRecordManager started")
	logCurrentNodeStatus("StartRetrieveNodesStatus - After start")
	return manager
}

// StartDefaultManager starts the default node status retrieval manager
// This should be called at system startup
func StartDefaultManager() {
	restartMu.Lock()
	defer restartMu.Unlock()

	logger.Debug("StartDefaultManager called")
	logCurrentNodeStatus("StartDefaultManager - Before start")

	if defaultManager != nil {
		logger.Info("DefaultManager already running, restarting...")
		logger.Debug("Default manager exists, performing restart")
		defaultManager.Restart()
		return
	}

	logger.Info("Starting default NodeRecordManager...")
	logger.Debug("No default manager exists, initializing new one")
	InitDefaultManager()

	logger.Debug("StartDefaultManager completed")
	logCurrentNodeStatus("StartDefaultManager - After start")
}

// cleanupDisabledNodes removes retry states for environments that are no longer enabled
func cleanupDisabledNodes(enabledEnvIDs []uint64) {
	retryMutex.Lock()
	defer retryMutex.Unlock()

	// Create a map for quick lookup
	enabledMap := make(map[uint64]bool)
	for _, id := range enabledEnvIDs {
		enabledMap[id] = true
	}

	// Remove retry states for disabled environments
	var cleanedUp []uint64
	for envID := range retryStates {
		if !enabledMap[envID] {
			delete(retryStates, envID)
			cleanedUp = append(cleanedUp, envID)
		}
	}

	if len(cleanedUp) > 0 {
		logger.Debugf("cleanupDisabledNodes: Cleaned up retry states for disabled environments: %v", cleanedUp)
	}
}

// removeFromNodeMap removes disabled nodes from NodeMap
func removeFromNodeMap(enabledEnvIDs []uint64) {
	mutex.Lock()
	defer mutex.Unlock()

	// Create a map for quick lookup
	enabledMap := make(map[uint64]bool)
	for _, id := range enabledEnvIDs {
		enabledMap[id] = true
	}

	// Remove nodes for disabled environments
	var removed []uint64
	for envID := range NodeMap {
		if !enabledMap[envID] {
			delete(NodeMap, envID)
			removed = append(removed, envID)
		}
	}

	if len(removed) > 0 {
		logger.Debugf("removeFromNodeMap: Removed disabled nodes from NodeMap: %v", removed)
	}
}

// checkEnvironmentStillEnabled checks if an environment is still enabled
func checkEnvironmentStillEnabled(envID uint64) bool {
	env := query.Environment
	environment, err := env.Where(env.ID.Eq(envID), env.Enabled.Is(true)).First()
	if err != nil {
		logger.Debugf("checkEnvironmentStillEnabled: Environment ID %d no longer enabled or not found", envID)
		return false
	}
	return environment != nil
}

func RetrieveNodesStatus(ctx context.Context) {
	logger.Info("RetrieveNodesStatus start")
	logger.Debug("RetrieveNodesStatus: Initializing node status retrieval")
	defer logger.Info("RetrieveNodesStatus exited")
	defer logger.Debug("RetrieveNodesStatus: Cleanup completed")

	mutex.Lock()
	if NodeMap == nil {
		logger.Debug("RetrieveNodesStatus: NodeMap is nil, creating new one")
		NodeMap = make(TNodeMap)
	} else {
		logger.Debugf("RetrieveNodesStatus: NodeMap already exists with %d nodes", len(NodeMap))
	}
	mutex.Unlock()

	logCurrentNodeStatus("RetrieveNodesStatus - Initial state")

	// Add periodic environment checking ticker
	envCheckTicker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer envCheckTicker.Stop()

	env := query.Environment
	envs, err := env.Where(env.Enabled.Is(true)).Find()
	if err != nil {
		logger.Error(err)
		logger.Debug("RetrieveNodesStatus: Failed to query enabled environments")
		return
	}

	logger.Debugf("RetrieveNodesStatus: Found %d enabled environments", len(envs))
	for i, e := range envs {
		logger.Debugf("RetrieveNodesStatus: Environment[%d] - ID: %d, Name: %s, Enabled: %t",
			i, e.ID, e.Name, e.Enabled)
	}

	// Get current enabled environment IDs
	var enabledEnvIDs []uint64
	for _, e := range envs {
		enabledEnvIDs = append(enabledEnvIDs, e.ID)
	}

	// Clean up disabled nodes
	cleanupDisabledNodes(enabledEnvIDs)
	removeFromNodeMap(enabledEnvIDs)

	var wg sync.WaitGroup
	defer wg.Wait()

	// Channel to signal when environment list changes
	envUpdateChan := make(chan []uint64, 1)

	// Start environment monitoring goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logger.Debug("RetrieveNodesStatus: Environment monitor goroutine completed")

		for {
			select {
			case <-ctx.Done():
				logger.Debug("RetrieveNodesStatus: Environment monitor context cancelled")
				return
			case <-envCheckTicker.C:
				// Re-check enabled environments
				currentEnvs, err := env.Where(env.Enabled.Is(true)).Find()
				if err != nil {
					logger.Error("RetrieveNodesStatus: Failed to re-query environments:", err)
					continue
				}

				var currentEnabledIDs []uint64
				for _, e := range currentEnvs {
					currentEnabledIDs = append(currentEnabledIDs, e.ID)
				}

				// Check if environment list changed
				if !equalUint64Slices(enabledEnvIDs, currentEnabledIDs) {
					logger.Debugf("RetrieveNodesStatus: Environment list changed from %v to %v", enabledEnvIDs, currentEnabledIDs)
					cleanupDisabledNodes(currentEnabledIDs)
					removeFromNodeMap(currentEnabledIDs)

					// Update the list
					enabledEnvIDs = currentEnabledIDs

					// Notify about the change
					select {
					case envUpdateChan <- currentEnabledIDs:
					default:
						// Non-blocking send
					}
				}
			}
		}
	}()

	for _, env := range envs {
		wg.Add(1)
		logger.Debugf("RetrieveNodesStatus: Starting goroutine for environment ID: %d, Name: %s", env.ID, env.Name)
		go func(e *model.Environment) {
			defer wg.Done()
			defer logger.Debugf("RetrieveNodesStatus: Goroutine completed for environment ID: %d", e.ID)

			// Retry ticker - check every 1 second but use backoff logic to determine actual retry
			retryTicker := time.NewTicker(1 * time.Second)
			defer retryTicker.Stop()

			for {
				select {
				case <-ctx.Done():
					logger.Debugf("RetrieveNodesStatus: Context cancelled for environment ID: %d", e.ID)
					return
				case newEnabledIDs := <-envUpdateChan:
					// Check if this environment is still enabled
					found := false
					for _, id := range newEnabledIDs {
						if id == e.ID {
							found = true
							break
						}
					}
					if !found {
						logger.Debugf("RetrieveNodesStatus: Environment ID %d has been disabled, stopping goroutine", e.ID)
						return
					}
				case <-retryTicker.C:
					// Double-check if environment is still enabled before retrying
					if !checkEnvironmentStillEnabled(e.ID) {
						logger.Debugf("RetrieveNodesStatus: Environment ID %d no longer enabled, stopping goroutine", e.ID)
						// Clean up retry state
						retryMutex.Lock()
						delete(retryStates, e.ID)
						retryMutex.Unlock()
						return
					}

					// Check if we should retry based on backoff logic
					if !shouldRetry(e.ID, defaultRetryConfig) {
						continue // Skip this iteration
					}

					logger.Debugf("RetrieveNodesStatus: Attempting connection to environment ID: %d", e.ID)
					if err := nodeAnalyticRecord(e, ctx); err != nil {
						logger.Error(err)
						logger.Debugf("RetrieveNodesStatus: Connection failed for environment ID: %d, error: %v", e.ID, err)
						markConnectionFailure(e.ID, defaultRetryConfig, err)
					} else {
						logger.Debugf("RetrieveNodesStatus: Connection successful for environment ID: %d", e.ID)
						markConnectionSuccess(e.ID)
					}
				}
			}
		}(env)
	}

	logger.Debug("RetrieveNodesStatus: All goroutines started, waiting for completion")
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

func nodeAnalyticRecord(env *model.Environment, ctx context.Context) error {
	logger.Debugf("nodeAnalyticRecord: Starting for environment ID: %d, Name: %s", env.ID, env.Name)

	scopeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	node, err := InitNode(env)

	mutex.Lock()
	NodeMap[env.ID] = node
	mutex.Unlock()

	logger.Debugf("nodeAnalyticRecord: Node initialized for environment ID: %d", env.ID)

	if err != nil {
		logger.Debugf("nodeAnalyticRecord: InitNode failed for environment ID: %d, error: %v", env.ID, err)
		return err
	}

	u, err := env.GetWebSocketURL("/api/analytic/intro")
	if err != nil {
		logger.Debugf("nodeAnalyticRecord: GetWebSocketURL failed for environment ID: %d, error: %v", env.ID, err)
		return err
	}

	logger.Debugf("nodeAnalyticRecord: Connecting to WebSocket URL: %s for environment ID: %d", u, env.ID)

	header := http.Header{}
	header.Set("X-Node-Secret", env.Token)

	dial := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}

	c, _, err := dial.Dial(u, header)
	if err != nil {
		logger.Debugf("nodeAnalyticRecord: WebSocket dial failed for environment ID: %d, error: %v", env.ID, err)
		return err
	}

	defer c.Close()
	logger.Debugf("nodeAnalyticRecord: WebSocket connection established for environment ID: %d", env.ID)

	go func() {
		<-scopeCtx.Done()
		logger.Debugf("nodeAnalyticRecord: Context cancelled, closing WebSocket for environment ID: %d", env.ID)
		_ = c.Close()
	}()

	messageCount := 0

	for {
		// Use json.RawMessage to handle both NodeStat and Node types
		var rawMsg json.RawMessage
		err = c.ReadJSON(&rawMsg)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Debugf("nodeAnalyticRecord: Unexpected WebSocket error for environment ID: %d, error: %v", env.ID, err)
				return err
			}
			logger.Debugf("nodeAnalyticRecord: WebSocket read completed for environment ID: %d", env.ID)
			return nil
		}

		messageCount++
		logger.Debugf("nodeAnalyticRecord: Received message #%d from environment ID: %d", messageCount, env.ID)

		mutex.Lock()
		if NodeMap[env.ID] != nil {
			// Try to unmarshal as complete Node first (contains both NodeInfo and NodeStat)
			var fullNode Node
			if err := json.Unmarshal(rawMsg, &fullNode); err == nil && fullNode.Version != "" {
				// Check if version has changed
				oldVersion := NodeMap[env.ID].Version
				if oldVersion != "" && oldVersion != fullNode.Version {
					logger.Infof("nodeAnalyticRecord: Version updated for environment ID: %d, from %s to %s",
						env.ID, oldVersion, fullNode.Version)
				}

				// This is a complete Node with version info - update everything
				NodeMap[env.ID].NodeInfo = fullNode.NodeInfo
				NodeMap[env.ID].NodeStat = fullNode.NodeStat
				// Ensure status and response time are set
				NodeMap[env.ID].NodeStat.Status = true
				NodeMap[env.ID].NodeStat.ResponseAt = time.Now()

				logger.Debugf("nodeAnalyticRecord: Updated complete Node info for environment ID: %d, Version: %s, Status: %t, ResponseAt: %v",
					env.ID, fullNode.Version, NodeMap[env.ID].NodeStat.Status, NodeMap[env.ID].NodeStat.ResponseAt)
			} else {
				// Fall back to NodeStat only
				var nodeStat NodeStat
				if err := json.Unmarshal(rawMsg, &nodeStat); err == nil {
					// set online
					nodeStat.Status = true
					nodeStat.ResponseAt = time.Now()

					NodeMap[env.ID].NodeStat = nodeStat
					logger.Debugf("nodeAnalyticRecord: Updated NodeStat for environment ID: %d, Status: %t, ResponseAt: %v",
						env.ID, nodeStat.Status, nodeStat.ResponseAt)
				} else {
					logger.Debugf("nodeAnalyticRecord: Failed to unmarshal message for environment ID: %d, error: %v", env.ID, err)
				}
			}
		} else {
			logger.Debugf("nodeAnalyticRecord: Warning - Node not found in NodeMap for environment ID: %d", env.ID)
		}
		mutex.Unlock()
	}
}
