package nginx_log

import (
	"io"
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

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