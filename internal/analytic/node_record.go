package analytic

import (
	"context"
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

// NewNodeRecordManager creates a new NodeRecordManager with the provided context
func NewNodeRecordManager(parentCtx context.Context) *NodeRecordManager {
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

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		RetrieveNodesStatus(m.ctx)
	}()
}

// Stop cancels the current context and waits for operations to complete
func (m *NodeRecordManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cancel()
	m.wg.Wait()
}

// Restart stops and then restarts the node status retrieval
func (m *NodeRecordManager) Restart() {
	m.Stop()

	// Create new context
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// Start retrieval with new context
	m.Start()
}

// For backward compatibility
var (
	defaultManager *NodeRecordManager
	restartMu      sync.Mutex
)

// InitDefaultManager initializes the default NodeRecordManager
func InitDefaultManager() {
	if defaultManager != nil {
		defaultManager.Stop()
	}
	defaultManager = NewNodeRecordManager(context.Background())
	defaultManager.Start()
}

// RestartRetrieveNodesStatus restarts the node status retrieval process
// Kept for backward compatibility
func RestartRetrieveNodesStatus() {
	restartMu.Lock()
	defer restartMu.Unlock()

	if defaultManager == nil {
		InitDefaultManager()
		return
	}

	defaultManager.Restart()
}

// StartRetrieveNodesStatus starts the node status retrieval with a custom context
func StartRetrieveNodesStatus(ctx context.Context) *NodeRecordManager {
	manager := NewNodeRecordManager(ctx)
	manager.Start()
	return manager
}

// StartDefaultManager starts the default node status retrieval manager
// This should be called at system startup
func StartDefaultManager() {
	restartMu.Lock()
	defer restartMu.Unlock()

	if defaultManager != nil {
		logger.Info("DefaultManager already running, restarting...")
		defaultManager.Restart()
		return
	}

	logger.Info("Starting default NodeRecordManager...")
	InitDefaultManager()
}

func RetrieveNodesStatus(ctx context.Context) {
	logger.Info("RetrieveNodesStatus start")
	defer logger.Info("RetrieveNodesStatus exited")

	mutex.Lock()
	if NodeMap == nil {
		NodeMap = make(TNodeMap)
	}
	mutex.Unlock()

	env := query.Environment
	envs, err := env.Where(env.Enabled.Is(true)).Find()
	if err != nil {
		logger.Error(err)
		return
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, env := range envs {
		wg.Add(1)
		go func(e *model.Environment) {
			defer wg.Done()
			retryTicker := time.NewTicker(5 * time.Second)
			defer retryTicker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := nodeAnalyticRecord(e, ctx); err != nil {
						logger.Error(err)
						mutex.Lock()
						if NodeMap[e.ID] != nil {
							NodeMap[e.ID].Status = false
						}
						mutex.Unlock()
						select {
						case <-retryTicker.C:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}(env)
	}
}

func nodeAnalyticRecord(env *model.Environment, ctx context.Context) error {
	scopeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	node, err := InitNode(env)

	mutex.Lock()
	NodeMap[env.ID] = node
	mutex.Unlock()

	if err != nil {
		return err
	}

	u, err := env.GetWebSocketURL("/api/analytic/intro")
	if err != nil {
		return err
	}

	header := http.Header{}

	header.Set("X-Node-Secret", env.Token)

	dial := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}

	c, _, err := dial.Dial(u, header)
	if err != nil {
		return err
	}

	defer c.Close()

	go func() {
		<-scopeCtx.Done()
		_ = c.Close()
	}()

	var nodeStat NodeStat

	for {
		err = c.ReadJSON(&nodeStat)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				return err
			}
			return nil
		}

		// set online
		nodeStat.Status = true
		nodeStat.ResponseAt = time.Now()

		mutex.Lock()
		NodeMap[env.ID].NodeStat = nodeStat
		mutex.Unlock()
	}
}
