package router

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/server/api"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

const (
	PageSize = 128 * 1024
)

type controlStruct struct {
	LogName      string `json:"log_name"`
	Type         string `json:"type"`
	ConfName     string `json:"conf_name"`
	ServerIdx    int    `json:"server_idx"`
	DirectiveIdx int    `json:"directive_idx"`
}

type nginxLogPageResp struct {
	Content string `json:"content"`
	Page    int64  `json:"page"`
}

func (h *Handler) GetNginxLogPage(c *gin.Context) {
	page := cast.ToInt64(c.Query("page"))
	if page < 0 {
		page = 0
	}

	var req controlStruct
	if !api.BindAndValid(c, &req) {
		return
	}

	l, err := getLog(h.Srv, req.LogName)
	if err != nil {
		c.JSON(http.StatusOK, nginxLogPageResp{})
		log.Println("error GetNginxLogPage getLog", err)
		return
	}

	f, err := os.Open(l.Path)
	if err != nil {
		c.JSON(http.StatusOK, nginxLogPageResp{})
		log.Println("error GetNginxLogPage open file", err)
		return
	}

	logFileStat, err := os.Stat(l.Path)
	if err != nil {
		c.JSON(http.StatusOK, nginxLogPageResp{})
		log.Println("error GetNginxLogPage stat", err)
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
		c.JSON(http.StatusOK, nginxLogPageResp{})
		log.Println("error GetNginxLogPage seek", err)
		return
	}

	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusOK, nginxLogPageResp{})
		log.Println("error GetNginxLogPage read buf", err)
		return
	}

	c.JSON(http.StatusOK, nginxLogPageResp{
		Page:    page,
		Content: string(buf[:n]),
	})
}

func getLog(s model.Service, name string) (l model.Log, err error) {
	if err := s.DB.Where("name = ?", name).First(&l).Error; err != nil {
		return model.Log{}, err
	}
	return l, err
}

func tailNginxLog(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error, s model.Service) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("tailNginxLog recovery", err)
			err = ws.WriteMessage(websocket.TextMessage, err.([]byte))
			if err != nil {
				log.Println(err)
				return
			}
			return
		}
	}()

	control := <-controlChan
	log.Println("control....", control)

	for {
		log, err := getLog(s, control.LogName)
		if err != nil {
			errChan <- err
			return
		}

		seek := tail.SeekInfo{
			Offset: 0,
			Whence: io.SeekEnd,
		}

		// Create a tail
		t, err := tail.TailFile(log.Path, tail.Config{Follow: true,
			ReOpen: true, Location: &seek})

		if err != nil {
			errChan <- errors.Wrap(err, "error NginxAccessLog Tail")
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
					errChan <- errors.Wrap(err, "error NginxAccessLog write message")
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
			log.Println("tailNginxLog recovery", err)
			return
		}
	}()

	for {
		msgType, payload, err := ws.ReadMessage()
		if err != nil {
			errChan <- errors.Wrap(err, "error NginxAccessLog read message")
			return
		}

		if msgType != websocket.TextMessage {
			errChan <- errors.New("error NginxAccessLog message type")
			return
		}

		var msg controlStruct
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			errChan <- errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal")
			return
		}
		controlChan <- msg
	}
}

func (h *Handler) NginxLog(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("[Error] NginxAccessLog Upgrade", err)
		return
	}
	defer ws.Close()

	errChan := make(chan error, 1)
	controlChan := make(chan controlStruct, 1)

	go tailNginxLog(ws, controlChan, errChan, h.Srv)
	go handleLogControl(ws, controlChan, errChan)

	if err = <-errChan; err != nil {
		log.Println(err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
}
