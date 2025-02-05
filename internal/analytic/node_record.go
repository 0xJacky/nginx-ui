package analytic

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	wg          sync.WaitGroup
	restartMu   sync.Mutex // Add mutex to prevent concurrent restarts
)

func RestartRetrieveNodesStatus() {
	restartMu.Lock() // Acquire lock before modifying shared resources
	defer restartMu.Unlock()

	// Cancel previous context to stop all operations
	cancel()

	// Wait for previous goroutines to finish
	wg.Wait()

	// Create new context for this run
	ctx, cancel = context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		RetrieveNodesStatus()
	}()
}

func RetrieveNodesStatus() {
	logger.Info("RetrieveNodesStatus start")
	defer logger.Info("RetrieveNodesStatus exited")

	mutex.Lock()
	NodeMap = make(TNodeMap)
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

	<-ctx.Done()
}

func nodeAnalyticRecord(env *model.Environment, ctx context.Context) error {
	scopeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger.Debug("nodeAnalyticRecord")
	node, err := InitNode(env)

	mutex.Lock()
	logger.Debug("lock")
	NodeMap[env.ID] = node
	mutex.Unlock()
	logger.Debug("unlock")

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
		logger.Debug("close")
	}()

	var nodeStat NodeStat

	defer func() {
		if NodeMap[env.ID] != nil {
			mutex.Lock()
			NodeMap[env.ID].Status = false
			mutex.Unlock()
		}
	}()

	for {
		err = c.ReadJSON(&nodeStat)
		if err != nil {
			return err
		}

		logger.Debug("nodeStat", nodeStat)

		// set online
		nodeStat.Status = true
		nodeStat.ResponseAt = time.Now()

		mutex.Lock()
		NodeMap[env.ID].NodeStat = nodeStat
		mutex.Unlock()
	}
}
