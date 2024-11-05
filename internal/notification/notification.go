package notification

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
	"sync"
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

func Info(title string, details string) {
	push(model.NotificationInfo, title, details)
}

func Error(title string, details string) {
	push(model.NotificationError, title, details)
}

func Warning(title string, details string) {
	push(model.NotificationWarning, title, details)
}

func Success(title string, details string) {
	push(model.NotificationSuccess, title, details)
}

func push(nType model.NotificationType, title string, details string) {
	n := query.Notification

	data := &model.Notification{
		Type:    nType,
		Title:   title,
		Details: details,
	}

	err := n.Create(data)
	if err != nil {
		logger.Error(err)
		return
	}
	broadcast(data)
}

func broadcast(data *model.Notification) {
	mutex.RLock()
	defer mutex.RUnlock()
	for _, evtChan := range clientMap {
		evtChan <- data
	}
}
