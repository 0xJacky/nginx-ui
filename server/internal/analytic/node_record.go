package analytic

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/gorilla/websocket"
	"github.com/opentracing/opentracing-go/log"
	"net/http"
	"time"
)

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

func nodeAnalyticLive(env *model.Environment, errChan chan error) {
	for {
		err := nodeAnalyticRecord(env)

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

func nodeAnalyticRecord(env *model.Environment) (err error) {
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

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}
		logger.Debugf("recv: %s %s", env.Name, message)

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
