package cosy

import (
	"github.com/0xJacky/Nginx-UI/api/cosy/map2struct"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (c *Ctx[T]) Custom(fx func(ctx *Ctx[T])) {
	if c.abort {
		return
	}
	errs := c.validate()

	if len(errs) > 0 {
		c.ctx.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Requested with wrong parameters",
			"errors":  errs,
		})
		return
	}

	c.beforeDecodeHook()

	for k := range c.Payload {
		c.SelectedFields = append(c.SelectedFields, k)
	}

	err := map2struct.WeakDecode(c.Payload, &c.Model)

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	c.beforeExecuteHook()

	fx(c)
}
