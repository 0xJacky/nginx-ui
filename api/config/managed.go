package config

import (
	"errors"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/ingress"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

// rejectManagedSiteFile refuses edits to a file an ingress generates (its
// site file or one of its upstream files) unless the request passes
// ?force=1, the same rule the site and upstream APIs apply. It responds and
// reports true when the request must stop.
func rejectManagedSiteFile(c *gin.Context, absPath string) bool {
	err := ingress.CheckManagedPath(absPath, isForced(c))
	if err == nil {
		return false
	}
	var cErr *cosy.Error
	if ingress.IsManagedError(err) && errors.As(err, &cErr) {
		c.JSON(http.StatusConflict, cErr)
		return true
	}
	cosy.ErrHandler(c, err)
	return true
}

func isForced(c *gin.Context) bool {
	return cast.ToBool(c.Query("force"))
}
