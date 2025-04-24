package nginx_log

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// PageSize defines the size of log chunks returned by the API
	PageSize = 128 * 1024
)

// controlStruct represents the request parameters for getting log content
type controlStruct struct {
	Type    string `json:"type"`     // Type of log: "access" or "error"
	LogPath string `json:"log_path"` // Path to the log file
}

// nginxLogPageResp represents the response format for log content
type nginxLogPageResp struct {
	Content string                 `json:"content"`         // Log content
	Page    int64                  `json:"page"`            // Current page number
	Error   *translation.Container `json:"error,omitempty"` // Error message if any
}

// GetNginxLogPage handles retrieving a page of log content from a log file
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
			Error: translation.C(err.Error()),
		})
		logger.Error(err)
		return
	}

	logFileStat, err := os.Stat(logPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: translation.C(err.Error()),
		})
		logger.Error(err)
		return
	}

	if !logFileStat.Mode().IsRegular() {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: translation.C("Log file %{log_path} is not a regular file. "+
				"If you are using nginx-ui in docker container, please refer to "+
				"https://nginxui.com/zh_CN/guide/config-nginx-log.html for more information.",
				map[string]any{
					"log_path": logPath,
				}),
		})
		logger.Errorf("log file is not a regular file: %s", logPath)
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
			Error: translation.C(err.Error()),
		})
		logger.Error(err)
		return
	}
	defer f.Close()

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

	// seek to the correct position in the file
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: translation.C(err.Error()),
		})
		logger.Error(err)
		return
	}

	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusInternalServerError, nginxLogPageResp{
			Error: translation.C(err.Error()),
		})
		logger.Error(err)
		return
	}

	c.JSON(http.StatusOK, nginxLogPageResp{
		Page:    page,
		Content: string(buf[:n]),
	})
}

// GetLogList returns a list of Nginx log files
func GetLogList(c *gin.Context) {
	filters := []func(*nginx_log.NginxLogCache) bool{}

	if logType := c.Query("type"); logType != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogCache) bool {
			return entry.Type == logType
		})
	}

	if name := c.Query("name"); name != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogCache) bool {
			return strings.Contains(entry.Name, name)
		})
	}

	if path := c.Query("path"); path != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogCache) bool {
			return strings.Contains(entry.Path, path)
		})
	}

	data := nginx_log.GetAllLogs(filters...)

	orderBy := c.DefaultQuery("sort_by", "name")
	sort := c.DefaultQuery("order", "desc")

	data = nginx_log.Sort(orderBy, sort, data)

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}
