package api

import (
	"errors"
	"net/http"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// SettingsSaveError answers a failed settings.Update. A value app.ini cannot
// store is input the caller can correct, so it is a 400 that keeps the cosy
// scope and code for the frontend; cosy.ErrHandler would report it as 500.
func SettingsSaveError(c *gin.Context, err error) {
	var cErr *cosy.Error
	if settings.IsUnstorableValue(err) && errors.As(err, &cErr) {
		c.AbortWithStatusJSON(http.StatusBadRequest, cErr)
		return
	}
	cosy.ErrHandler(c, err)
}
