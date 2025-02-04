package analytic

import (
	"context"
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
	"net/http"
	"time"
)

var stopNodeRecordChan = make(chan struct{})

func RestartRetrieveNodesStatus() {
	stopNodeRecordChan <- struct{}{}
	time.Sleep(5 * time.Second)
	go RetrieveNodesStatus()
}

func RetrieveNodesStatus() {
	NodeMap = make(TNodeMap)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := query.Environment

	envs, err := env.Where(env.Enabled.Is(true)).Find()
	if err != nil {
		logger.Error(err)
		return
	}

	for _, v := range envs {
		go nodeAnalyticLive(v, ctx)
	}

	<-stopNodeRecordChan
	logger.Info("RetrieveNodesStatus exited normally")
	return // will execute defer cancel()
}

func nodeAnalyticLive(env *model.Environment, ctx context.Context) {
	errChan := make(chan error)
	for {
		nodeAnalyticRecord(env, errChan, ctx)

		select {
		case err := <-errChan:
			if NodeMap[env.ID] != nil {
				mutex.Lock()
				NodeMap[env.ID].Status = false
				mutex.Unlock()
			}
			logger.Error(err)
			// wait 5s then reconnect
			time.Sleep(5 * time.Second)
		case <-ctx.Done():
			return
		}
	}
}

func nodeAnalyticRecord(env *model.Environment, errChan chan error, ctx context.Context) {
	mutex.Lock()
	NodeMap[env.ID] = InitNode(env)
	mutex.Unlock()

	u, err := env.GetWebSocketURL("/api/analytic/intro")
	if err != nil {
		errChan <- err
		return
	}

	header := http.Header{}

	header.Set("X-Node-Secret", env.Token)

	dial := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}

	c, _, err := dial.Dial(u, header)
	if err != nil {
		errChan <- err
		return
	}

	defer c.Close()

	var nodeStat NodeStat

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if helper.IsUnexpectedWebsocketError(err) {
				errChan <- err
				return
			}

			err = json.Unmarshal(message, &nodeStat)
			if err != nil {
				errChan <- err
				return
			}

			// set online
			nodeStat.Status = true
			nodeStat.ResponseAt = time.Now()

			mutex.Lock()
			NodeMap[env.ID].NodeStat = nodeStat
			mutex.Unlock()
		}
	}()

	// shutdown
	<-ctx.Done()
	_ = c.Close()
}
