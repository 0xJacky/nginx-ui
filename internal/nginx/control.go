package nginx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type ControlFunc func() (stdOut string, stdErr error)

type ControlResult struct {
	stdOut string
	stdErr error
}

type ControlResp struct {
	Message string `json:"message"`
	Level   int    `json:"level"`
}

func Control(controlFunc ControlFunc) *ControlResult {
	stdout, stderr := controlFunc()
	return &ControlResult{
		stdOut: stdout,
		stdErr: stderr,
	}
}

func (t *ControlResult) IsError() bool {
	return GetLogLevel(t.stdOut) > Warn || t.stdErr != nil
}

func (t *ControlResult) Resp(c *gin.Context) {
	if t.IsError() {
		t.RespError(c)
		return
	}
	c.JSON(http.StatusOK, ControlResp{
		Message: t.stdOut,
		Level:   GetLogLevel(t.stdOut),
	})
}

func (t *ControlResult) RespError(c *gin.Context) {
	cosy.ErrHandler(c,
		cosy.WrapErrorWithParams(ErrNginx, strings.Join([]string{t.stdOut, t.stdErr.Error()}, " ")))
}

func (t *ControlResult) GetOutput() string {
	if t.stdErr == nil {
		return t.stdOut
	}
	return strings.Join([]string{t.stdOut, t.stdErr.Error()}, " ")
}

func (t *ControlResult) GetError() error {
	return errors.New(t.GetOutput())
}

func (t *ControlResult) GetLevel() int {
	return GetLogLevel(t.stdOut)
}
