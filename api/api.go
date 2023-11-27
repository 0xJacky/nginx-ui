package api

import (
	"errors"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/gin-gonic/gin"
	val "github.com/go-playground/validator/v10"
	"net/http"
	"reflect"
)

func ErrHandler(c *gin.Context, err error) {
	logger.GetLogger().Errorln(err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": err.Error(),
	})
}

type ValidError struct {
	Key     string
	Message string
}

func BindAndValid(c *gin.Context, target interface{}) bool {
	errs := make(map[string]string)
	err := c.ShouldBindJSON(target)
	if err != nil {
		logger.Error("bind err", err)

		var verrs val.ValidationErrors
		ok := errors.As(err, &verrs)

		if !ok {
			logger.Error("valid err", verrs)
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "Requested with wrong parameters",
				"code":    http.StatusNotAcceptable,
				"error":   verrs,
			})
			return false
		}

		for _, value := range verrs {
			t := reflect.ValueOf(target)
			realType := t.Type().Elem()
			field, _ := realType.FieldByName(value.StructField())
			errs[field.Tag.Get("json")] = value.Tag()
		}

		c.JSON(http.StatusNotAcceptable, gin.H{
			"errors":  errs,
			"message": "Requested with wrong parameters",
			"code":    http.StatusNotAcceptable,
		})

		return false
	}

	return true
}
