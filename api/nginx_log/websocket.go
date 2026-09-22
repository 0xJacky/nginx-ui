package nginx_log

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nxadm/tail"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// getLogPath resolves the log file path based on the provided control parameters
// It checks if the path is under the whitelist directories
func getLogPath(control *controlStruct) (logPath string, err error) {
	// If direct log path is provided, use it
	if control.Path != "" {
		logPath = control.Path
		// Check if logPath is under one of the paths in LogDirWhiteList
		if !utils.IsValidLogPath(logPath) {
			return "", nginx_log.ErrLogPathIsNotUnderTheLogDirWhiteList
		}
		return
	}

	// Otherwise, use default log path based on type
	switch control.Type {
	case "error":
		path := nginx.GetErrorLogPath()

		if path == "" {
			err = nginx_log.ErrErrorLogPathIsEmpty
			return
		}

		logPath = path
	case "access":
		fallthrough
	default:
		path := nginx.GetAccessLogPath()

		if path == "" {
			err = nginx_log.ErrAccessLogPathIsEmpty
			return
		}

		logPath = path
	}

	// check if logPath is under one of the paths in LogDirWhiteList
	if !utils.IsValidLogPath(logPath) {
		return "", nginx_log.ErrLogPathIsNotUnderTheLogDirWhiteList
	}
	return
}

// reportLogError hands err, or nil for a quiet end, to the session without
// blocking. The channel has room for one report from each session goroutine,
// so a full channel only means the session is already ending.
func reportLogError(errChan chan<- error, err error) {
	select {
	case errChan <- err:
	default:
	}
}

// reportLogWriteError ends the session after a failed WebSocket write, logging
// only failures other than the peer going away.
func reportLogWriteError(errChan chan<- error, err error, message string) {
	if helper.IsUnexpectedWebsocketError(err) {
		reportLogError(errChan, errors.Wrap(err, message))
		return
	}
	reportLogError(errChan, nil)
}

// waitForLogControl waits for the client to choose a log. It reports false
// when the session ends first.
func waitForLogControl(done <-chan struct{}, controlChan <-chan controlStruct) (controlStruct, bool) {
	select {
	case control := <-controlChan:
		return control, true
	case <-done:
		return controlStruct{}, false
	}
}

// tailNginxLog tails the specified log file and sends each line to the websocket
// until done is closed.
func tailNginxLog(done <-chan struct{}, writer *helper.SafeWebSocketWriter, controlChan <-chan controlStruct, errChan chan<- error) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
			return
		}
	}()

	usesSFTP, err := nginx.UsesSFTPTarget()
	if err != nil {
		reportLogError(errChan, err)
		return
	}
	if usesSFTP {
		tailSFTPNginxLog(done, writer, controlChan, errChan)
		return
	}
	tailLocalNginxLog(done, writer, controlChan, errChan)
}

func tailLocalNginxLog(done <-chan struct{}, writer *helper.SafeWebSocketWriter, controlChan <-chan controlStruct, errChan chan<- error) {
	control, ok := waitForLogControl(done, controlChan)
	if !ok {
		return
	}

	for {
		logPath, err := getLogPath(&control)

		if err != nil {
			reportLogError(errChan, err)
			return
		}

		seek := tail.SeekInfo{
			Offset: 0,
			Whence: io.SeekEnd,
		}

		stat, err := nginx.Stat(logPath)
		if os.IsNotExist(err) {
			reportLogError(errChan, cosy.WrapErrorWithParams(nginx_log.ErrLogFileNotExists, logPath))
			return
		}
		if err != nil {
			reportLogError(errChan, err)
			return
		}

		if !stat.Mode().IsRegular() {
			reportLogError(errChan, cosy.WrapErrorWithParams(nginx_log.ErrLogFileNotRegular, logPath))
			return
		}

		// Create a tail
		t, err := tail.TailFile(logPath, tail.Config{Follow: true,
			ReOpen: true, Location: &seek})
		if err != nil {
			reportLogError(errChan, errors.Wrap(err, "error tailing log"))
			return
		}

		control, ok = followLocalLog(done, t, writer, controlChan, errChan)
		if !ok {
			return
		}
	}
}

// followLocalLog sends the lines t produces until the client selects another
// log, which it returns with true. It returns false once the session is over.
// The tail is stopped on every return, so its file watcher never outlives the
// session or survives a switch to another log.
func followLocalLog(done <-chan struct{}, t *tail.Tail, writer *helper.SafeWebSocketWriter, controlChan <-chan controlStruct, errChan chan<- error) (controlStruct, bool) {
	defer func() {
		_ = t.Stop()
	}()

	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				// The tail gave up on its own, e.g. the file became unreadable.
				reportLogError(errChan, errors.Wrap(t.Err(), "error tailing log"))
				return controlStruct{}, false
			}
			if line == nil {
				continue
			}

			if err := writer.WriteMessage(websocket.TextMessage, []byte(line.Text)); err != nil {
				reportLogWriteError(errChan, err, "error tailNginxLog write message")
				return controlStruct{}, false
			}
		case control := <-controlChan:
			return control, true
		case <-done:
			return controlStruct{}, false
		}
	}
}

func tailSFTPNginxLog(done <-chan struct{}, writer *helper.SafeWebSocketWriter, controlChan <-chan controlStruct, errChan chan<- error) {
	control, ok := waitForLogControl(done, controlChan)
	if !ok {
		return
	}

	for {
		logPath, err := getLogPath(&control)
		if err != nil {
			reportLogError(errChan, err)
			return
		}

		stat, err := nginx.Stat(logPath)
		if os.IsNotExist(err) {
			reportLogError(errChan, cosy.WrapErrorWithParams(nginx_log.ErrLogFileNotExists, logPath))
			return
		}
		if err != nil {
			reportLogError(errChan, err)
			return
		}
		if !stat.Mode().IsRegular() {
			reportLogError(errChan, cosy.WrapErrorWithParams(nginx_log.ErrLogFileNotRegular, logPath))
			return
		}

		file, err := nginx.Open(logPath)
		if err != nil {
			reportLogError(errChan, err)
			return
		}
		offset := stat.Size()
		var pending []byte
		ticker := time.NewTicker(500 * time.Millisecond)
		next := false

		for !next {
			select {
			case control = <-controlChan:
				next = true
			case <-done:
				ticker.Stop()
				_ = file.Close()
				return
			case <-ticker.C:
				current, statErr := nginx.Stat(logPath)
				if statErr != nil {
					ticker.Stop()
					_ = file.Close()
					reportLogError(errChan, statErr)
					return
				}
				if current.Size() < offset {
					_ = file.Close()
					file, err = nginx.Open(logPath)
					if err != nil {
						ticker.Stop()
						reportLogError(errChan, err)
						return
					}
					offset = 0
					pending = nil
				}
				if current.Size() == offset {
					continue
				}
				if _, err = file.Seek(offset, io.SeekStart); err != nil {
					ticker.Stop()
					_ = file.Close()
					reportLogError(errChan, err)
					return
				}
				chunk, readErr := io.ReadAll(io.LimitReader(file, current.Size()-offset))
				if readErr != nil {
					ticker.Stop()
					_ = file.Close()
					reportLogError(errChan, readErr)
					return
				}
				offset += int64(len(chunk))
				pending = append(pending, chunk...)
				lines := bytes.Split(pending, []byte{'\n'})
				pending = append(pending[:0], lines[len(lines)-1]...)
				for _, line := range lines[:len(lines)-1] {
					if err = writer.WriteMessage(websocket.TextMessage, line); err != nil {
						ticker.Stop()
						_ = file.Close()
						reportLogWriteError(errChan, err, "error tailSFTPNginxLog write message")
						return
					}
				}
			}
		}

		ticker.Stop()
		_ = file.Close()
	}
}

// handleLogControl processes websocket control messages. It is the
// connection's only reader, so it reads through the keepalive.
func handleLogControl(done <-chan struct{}, keepalive *helper.WebSocketKeepalive, controlChan chan<- controlStruct, errChan chan<- error) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
			return
		}
	}()

	for {
		msgType, payload, err := keepalive.ReadMessage()
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				reportLogError(errChan, errors.Wrap(err, "error handleLogControl read message"))
				return
			}
			// The client closed the connection or stopped answering pings.
			reportLogError(errChan, nil)
			return
		}

		if msgType != websocket.TextMessage {
			reportLogError(errChan, nginx_log.ErrInvalidWebSocketMessageType)
			return
		}

		var msg controlStruct
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			reportLogError(errChan, errors.Wrap(err, "error ReadWsAndWritePty json.Unmarshal"))
			return
		}

		select {
		case controlChan <- msg:
		case <-done:
			return
		}
	}
}

// Log handles websocket connection for real-time log viewing
func Log(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer ws.Close()

	// Arm the keepalive before handleLogControl starts reading. A client that
	// vanished without a close frame then ends the session, and the tail it
	// drives, within the pong wait.
	keepalive := helper.StartWebSocketKeepalive(ws)
	defer keepalive.Stop()

	wsWriter := helper.NewSafeWebSocketWriter(ws)

	// Room for one report from each session goroutine, so neither blocks.
	errChan := make(chan error, 2)
	controlChan := make(chan controlStruct, 1)

	// Closing done stops the tail and control goroutines with the session.
	done := make(chan struct{})
	defer close(done)

	go tailNginxLog(done, wsWriter, controlChan, errChan)
	go handleLogControl(done, keepalive, controlChan, errChan)

	if err = <-errChan; err != nil {
		logger.Error(err)
		_ = wsWriter.WriteMessage(websocket.TextMessage, []byte(err.Error()))
	}
}
