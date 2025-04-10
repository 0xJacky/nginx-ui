package notification

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
)

var (
	clientMap = make(map[*gin.Context]chan *model.Notification)
	mutex     = &sync.RWMutex{}
)

func SetClient(c *gin.Context, evtChan chan *model.Notification) {
	mutex.Lock()
	defer mutex.Unlock()
	clientMap[c] = evtChan
}

func RemoveClient(c *gin.Context) {
	mutex.Lock()
	defer mutex.Unlock()
	close(clientMap[c])
	delete(clientMap, c)
}

func broadcast(data *model.Notification) {
	mutex.RLock()
	defer mutex.RUnlock()
	for _, evtChan := range clientMap {
		evtChan <- data
	}
}
