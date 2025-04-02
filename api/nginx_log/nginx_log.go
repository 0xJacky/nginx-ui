package nginx_log

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	PageSize = 128 * 1024
)

type controlStruct struct {
	Type    string `json:"type"`
	LogPath string `json:"log_path"`
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

func GetLogList(c *gin.Context) {
	filters := []func(*cache.NginxLogCache) bool{}

	if c.Query("type") != "" {
		filters = append(filters, func(cache *cache.NginxLogCache) bool {
			return cache.Type == c.Query("type")
		})
	}

	if c.Query("name") != "" {
		filters = append(filters, func(cache *cache.NginxLogCache) bool {
			return strings.Contains(cache.Name, c.Query("name"))
		})
	}

	if c.Query("path") != "" {
		filters = append(filters, func(cache *cache.NginxLogCache) bool {
			return strings.Contains(cache.Path, c.Query("path"))
		})
	}

	data := cache.GetAllLogPaths(filters...)

	orderBy := c.DefaultQuery("sort_by", "name")
	sort := c.DefaultQuery("order", "desc")

	data = nginx_log.Sort(orderBy, sort, data)

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}
