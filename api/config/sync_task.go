package config

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy/logger"
)

type configBatchTask struct {
	done   chan struct{}
	result batchResult
}

var (
	batchTasks sync.Map
	// Each queued task holds its request body until it obtains the apply lock.
	batchSlots = make(chan struct{}, 4)
)

func startBatchTask(c *gin.Context, batch syncBatchRequest, force bool) {
	select {
	case batchSlots <- struct{}{}:
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"message": "the node is already processing configuration batches"})
		return
	}
	id := uuid.NewString()
	task := &configBatchTask{done: make(chan struct{})}
	batchTasks.Store(id, task)
	go func() {
		defer func() {
			if failure := recover(); failure != nil {
				logger.Errorf("config batch %s panicked: %v", id, failure)
				task.result = batchResult{err: errors.New("configuration batch failed unexpectedly")}
			}
			close(task.done)
			<-batchSlots
			time.AfterFunc(10*time.Minute, func() { batchTasks.Delete(id) })
		}()
		task.result = applyConfigBatch(batch, force)
	}()
	c.JSON(http.StatusAccepted, gin.H{"id": id})
}

func GetConfigBatchTask(c *gin.Context) {
	value, ok := batchTasks.Load(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "unknown or expired configuration batch"})
		return
	}
	c.Header("Cache-Control", "no-store")
	task := value.(*configBatchTask)
	select {
	case <-task.done:
		task.result.respond(c)
	default:
		c.JSON(http.StatusAccepted, gin.H{"id": c.Param("id")})
	}
}
