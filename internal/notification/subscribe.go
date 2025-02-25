package notification

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
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

func push(nType model.NotificationType, title string, content string, details any) {
	n := query.Notification

	data := &model.Notification{
		Type:    nType,
		Title:   title,
		Content: content,
		Details: details,
	}

	err := n.Create(data)
	if err != nil {
		logger.Error(err)
		return
	}
	broadcast(data)
}
