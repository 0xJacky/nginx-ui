package upstream

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

type wsMessage struct {
	data interface{}
	done chan error
}

func AvailabilityTest(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer ws.Close()

	var currentTargets []string
	var targetsMutex sync.RWMutex

	// Use context to manage goroutine lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use channel to serialize WebSocket write operations, avoiding concurrent conflicts
	writeChan := make(chan wsMessage, 10)
	testChan := make(chan bool, 1) // Immediate test signal

	// Create debouncer for test execution
	testDebouncer := helper.NewDebouncer(300 * time.Millisecond)

	// WebSocket writer goroutine - serialize all write operations
	go func() {
		defer logger.Debug("WebSocket writer goroutine stopped")
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-writeChan:
				err := ws.WriteJSON(msg.data)
				if msg.done != nil {
					msg.done <- err
					close(msg.done)
				}
				if err != nil {
					logger.Error("Failed to send WebSocket message:", err)
					if helper.IsUnexpectedWebsocketError(err) {
						cancel() // Cancel all goroutines
					}
				}
			}
		}
	}()

	// Safe WebSocket write function
	writeJSON := func(data interface{}) error {
		done := make(chan error, 1)
		msg := wsMessage{data: data, done: done}

		select {
		case writeChan <- msg:
			return <-done
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second): // Prevent write blocking
			return context.DeadlineExceeded
		}
	}

	// Function to perform availability test
	performTest := func() {
		targetsMutex.RLock()
		targets := make([]string, len(currentTargets))
		copy(targets, currentTargets)
		targetsMutex.RUnlock()

		logger.Debug("Performing availability test for targets:", targets)

		if len(targets) > 0 {
			logger.Debug("Starting upstream.AvailabilityTest...")
			result := upstream.AvailabilityTest(targets)
			logger.Debug("Test completed, results:", result)

			logger.Debug("Sending results via WebSocket...")
			if err := writeJSON(result); err != nil {
				logger.Error("Failed to send WebSocket message:", err)
				if helper.IsUnexpectedWebsocketError(err) {
					cancel() // Cancel all goroutines
				}
			} else {
				logger.Debug("Results sent successfully")
			}
		} else {
			logger.Debug("No targets to test")
			// Send empty result even if no targets
			emptyResult := make(map[string]interface{})
			if err := writeJSON(emptyResult); err != nil {
				logger.Error("Failed to send empty result:", err)
			} else {
				logger.Debug("Empty result sent successfully")
			}
		}
	}

	// Goroutine to handle incoming messages (target updates)
	go func() {
		defer logger.Debug("WebSocket reader goroutine stopped")
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var newTargets []string
			// Set read timeout to avoid blocking
			ws.SetReadDeadline(time.Now().Add(30 * time.Second))
			err := ws.ReadJSON(&newTargets)
			ws.SetReadDeadline(time.Time{}) // Clear deadline

			if err != nil {
				if helper.IsUnexpectedWebsocketError(err) {
					logger.Error(err)
				}
				cancel() // Cancel all goroutines
				return
			}

			logger.Debug("Received targets from frontend:", newTargets)

			targetsMutex.Lock()
			currentTargets = newTargets
			targetsMutex.Unlock()

			// Use debouncer to trigger test execution
			testDebouncer.Trigger(func() {
				select {
				case testChan <- true:
				default:
				}
			})
		}
	}()

	// Main testing loop
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	logger.Debug("WebSocket connection established, waiting for messages...")

	for {
		select {
		case <-ctx.Done():
			testDebouncer.Stop()
			logger.Debug("WebSocket connection closed")
			return
		case <-testChan:
			// Debounce triggered test or first test
			go performTest() // Execute asynchronously to avoid blocking main loop
		case <-ticker.C:
			// Periodic test execution
			go performTest() // Execute asynchronously to avoid blocking main loop
		}
	}
}
