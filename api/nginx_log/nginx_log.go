package nginx_log

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	PageSize = 128 * 1024
)

type controlStruct struct {
	Type         string `json:"type"`
	ConfName     string `json:"conf_name"`
	ServerIdx    int    `json:"server_idx"`
	DirectiveIdx int    `json:"directive_idx"`
}

type nginxLogPageResp struct {
	Content string `json:"content"`
	Page    int64  `json:"page"`
	Error   string `json:"error,omitempty"`
}

func GetNginxLogPage(c *gin.Context) {
	page := cast.ToInt64(c.Query("page"))
	if page < 0 {
		page = 0
	}

	var control controlStruct
	if !cosy.BindAndValid(c, &control) {
		return
	}

	logPath, err := getLogPath(&control)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: err.Error(),
		})
		logger.Error(err)
		return
	}

	logFileStat, err := os.Stat(logPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: err.Error(),
		})
		logger.Error(err)
		return
	}

	if !logFileStat.Mode().IsRegular() {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: "log file is not regular file",
		})
		logger.Errorf("log file is not regular file: %s", logPath)
		return
	}

	// to fix: seek invalid argument #674
	if logFileStat.Size() == 0 {
		c.JSON(http.StatusOK, nginxLogPageResp{
			Page:    1,
			Content: "",
		})
		return
	}

	f, err := os.Open(logPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: err.Error(),
		})
		logger.Error(err)
		return
	}

	totalPage := logFileStat.Size() / PageSize

	if logFileStat.Size()%PageSize > 0 {
		totalPage++
	}

	var buf []byte
	var offset int64
	if page == 0 {
		page = totalPage
	}

	buf = make([]byte, PageSize)
	offset = (page - 1) * PageSize

	// seek
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: err.Error(),
		})
		logger.Error(err)
		return
	}

	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: err.Error(),
		})
		logger.Error(err)
		return
	}

	c.JSON(http.StatusOK, nginxLogPageResp{
		Page:    page,
		Content: string(buf[:n]),
	})
}

func getLogPath(control *controlStruct) (logPath string, err error) {
	switch control.Type {
	case "site":
		var config *nginx.NgxConfig
		path := nginx.GetConfPath("sites-available", control.ConfName)
		config, err = nginx.ParseNgxConfig(path)
		if err != nil {
			err = errors.Wrap(err, "error parsing ngx config")
			return
		}

		if control.ServerIdx >= len(config.Servers) {
			err = nginx_log.ErrServerIdxOutOfRange
			return
		}

		if control.DirectiveIdx >= len(config.Servers[control.ServerIdx].Directives) {
			err = nginx_log.ErrDirectiveIdxOutOfRange
			return
		}

		directive := config.Servers[control.ServerIdx].Directives[control.DirectiveIdx]
		switch directive.Directive {
		case "access_log", "error_log":
			// ok
		default:
			err = nginx_log.ErrLogDirective
			return
		}

		if directive.Params == "" {
			err = nginx_log.ErrDirectiveParamsIsEmpty
			return
		}

		// fix: access_log /var/log/test.log main;
		p := strings.Split(directive.Params, " ")
		if len(p) > 0 {
			logPath = p[0]
		}

	case "error":
		path := nginx.GetErrorLogPath()

		if path == "" {
			err = nginx_log.ErrErrorLogPathIsEmpty
			return
		}

		logPath = path
	default:
		path := nginx.GetAccessLogPath()

		if path == "" {
			err = nginx_log.ErrAccessLogPathIsEmpty
			return
		}

		logPath = path
	}

	// check if logPath is under one of the paths in LogDirWhiteList
	if !nginx_log.IsLogPathUnderWhiteList(logPath) {
		return "", nginx_log.ErrLogPathIsNotUnderTheLogDirWhiteList
	}
	return
}

func tailNginxLog(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
			return
		}
	}()

	control := <-controlChan

	for {
		logPath, err := getLogPath(&control)

		if err != nil {
			errChan <- err
			return
		}

		seek := tail.SeekInfo{
			Offset: 0,
			Whence: io.SeekEnd,
		}

		stat, err := os.Stat(logPath)
		if os.IsNotExist(err) {
			errChan <- errors.New("[error] log path not exists " + logPath)
			return
		}

		if !stat.Mode().IsRegular() {
			errChan <- errors.New("[error] " + logPath + " is not a regular file. " +
				"If you are using nginx-ui in docker container, please refer to " +
				"https://nginxui.com/zh_CN/guide/config-nginx-log.html for more information.")
			return
		}

		// Create a tail
		t, err := tail.TailFile(logPath, tail.Config{Follow: true,
			ReOpen: true, Location: &seek})

		if err != nil {
			errChan <- errors.Wrap(err, "error tailing log")
			return
		}

		for {
			var next = false
			select {
			case line := <-t.Lines:
				// Print the text of each received line
				if line == nil {
					continue
				}

				err = ws.WriteMessage(websocket.TextMessage, []byte(line.Text))
				if err != nil {
					if helper.IsUnexpectedWebsocketError(err) {
						errChan <- errors.Wrap(err, "error tailNginxLog write message")
					}
					return
				}
			case control = <-controlChan:
				next = true
				break
			}
			if next {
				break
			}
		}
	}
}

func handleLogControl(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
			return
		}
	}()

	for {
		msgType, payload, err := ws.ReadMessage()
		if err != nil && websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
			errChan <- errors.Wrap(err, "error handleLogControl read message")
			return
		}

		if msgType != websocket.TextMessage {
			errChan <- errors.New("error handleLogControl message type")
			return
		}

		var msg controlStruct
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			errChan <- errors.Wrap(err, "error ReadWsAndWritePty json.Unmarshal")
			return
		}
		controlChan <- msg
	}
}

func Log(c *gin.Context) {
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

	errChan := make(chan error, 1)
	controlChan := make(chan controlStruct, 1)

	go tailNginxLog(ws, controlChan, errChan)
	go handleLogControl(ws, controlChan, errChan)

	if err = <-errChan; err != nil {
		logger.Error(err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
}
