package nginx

import (
	"encoding/json"
	"fmt"
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
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
	if !api.BindAndValid(c, &control) {
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
		logger.Error("log file is not regular file:", logPath)
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

	if err != nil && err != io.EOF {
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

// isLogPathUnderWhiteList checks if the log path is under one of the paths in LogDirWhiteList
func isLogPathUnderWhiteList(path string) bool {
	cacheKey := fmt.Sprintf("isLogPathUnderWhiteList:%s", path)
	res, ok := cache.Get(cacheKey)
	// no cache, check it
	if !ok {
		for _, whitePath := range settings.NginxSettings.LogDirWhiteList {
			if helper.IsUnderDirectory(path, whitePath) {
				cache.Set(cacheKey, true, 0)
				return true
			}
		}
		return false
	}
	return res.(bool)
}

func getLogPath(control *controlStruct) (logPath string, err error) {
	if len(settings.NginxSettings.LogDirWhiteList) == 0 {
		err = errors.New("The settings.NginxSettings.LogDirWhiteList has not been configured. " +
			"For security reasons, please configure a whitelist of log directories. " +
			"Please visit https://nginxui.com/guide/config-nginx.html for more information.")
		return
	}
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
			err = errors.New("serverIdx out of range")
			return
		}

		if control.DirectiveIdx >= len(config.Servers[control.ServerIdx].Directives) {
			err = errors.New("DirectiveIdx out of range")
			return
		}

		directive := config.Servers[control.ServerIdx].Directives[control.DirectiveIdx]
		switch directive.Directive {
		case "access_log", "error_log":
			// ok
		default:
			err = errors.New("directive.Params neither access_log nor error_log")
			return
		}

		if directive.Params == "" {
			err = errors.New("directive.Params is empty")
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
			err = errors.New("settings.NginxLogSettings.ErrorLogPath is empty," +
				" refer to https://nginxui.com/guide/config-nginx.html for more information")
			return
		}

		logPath = path
	default:
		path := nginx.GetAccessLogPath()

		if path == "" {
			err = errors.New("settings.NginxLogSettings.AccessLogPath is empty," +
				" refer to https://nginxui.com/guide/config-nginx.html for more information")
			return
		}

		logPath = path
	}

	// check if logPath is under one of the paths in LogDirWhiteList
	if !isLogPathUnderWhiteList(logPath) {
		err = errors.New("The log path is not under the paths in LogDirWhiteList.")
		return "", err
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

				if err != nil && websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
					errChan <- errors.Wrap(err, "error tailNginxLog write message")
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
