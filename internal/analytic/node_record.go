package analytic

import (
	"context"
	"encoding/json"
	"github.com/uozi-tech/cosy/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

var stopNodeRecordChan = make(chan struct{})

func RestartRetrieveNodesStatus() {
	stopNodeRecordChan <- struct{}{}
	time.Sleep(10 * time.Second)
	go RetrieveNodesStatus()
}

func RetrieveNodesStatus() {
	NodeMap = make(TNodeMap)
	errChan := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	env := query.Environment

	envs, err := env.Where(env.Enabled.Is(true)).Find()
	if err != nil {
		logger.Error(err)
		return
	}

	for _, v := range envs {
		go nodeAnalyticLive(v, errChan, ctx)
	}

	for {
		select {
		case err = <-errChan:
			logger.Error(err)
		case <-stopNodeRecordChan:
			logger.Info("RetrieveNodesStatus exited normally")
			return // will execute defer cancel()
		}
	}
}

func nodeAnalyticLive(env *model.Environment, errChan chan error, ctx context.Context) {
	for {
		err := nodeAnalyticRecord(env, ctx)

		if err != nil {
			// set node offline
			if NodeMap[env.ID] != nil {
				mutex.Lock()
				NodeMap[env.ID].Status = false
				mutex.Unlock()
			}
			logger.Error(err)
			errChan <- err
			// wait 5s then reconnect
			time.Sleep(5 * time.Second)
		}
	}
}

func nodeAnalyticRecord(env *model.Environment, ctx context.Context) (err error) {
	mutex.Lock()
	NodeMap[env.ID] = InitNode(env)
	mutex.Unlock()

	u, err := env.GetWebSocketURL("/api/analytic/intro")

	if err != nil {
		return
	}

	header := http.Header{}

	header.Set("X-Node-Secret", env.Token)

	c, _, err := websocket.DefaultDialer.Dial(u, header)
	if err != nil {
		return
	}

	defer c.Close()

	var nodeStat NodeStat

	go func() {
		// shutdown
		<-ctx.Done()
		_ = c.Close()
	}()

	for {
		_, message, err := c.ReadMessage()
		if err != nil || websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNoStatusReceived,
			websocket.CloseNormalClosure) {
			return err
		}

		err = json.Unmarshal(message, &nodeStat)

		if err != nil {
			return err
		}

		// set online
		nodeStat.Status = true
		nodeStat.ResponseAt = time.Now()

		mutex.Lock()
		NodeMap[env.ID].NodeStat = nodeStat
		mutex.Unlock()
	}
}
