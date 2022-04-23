package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	val "github.com/go-playground/validator/v10"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"log"
	"net/http"
	"reflect"
)

func ErrHandler(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": err.Error(),
	})
}

type ValidError struct {
	Key     string
	Message string
}

var trans ut.Translator

func init() {
	uni := ut.New(zh.New())
	trans, _ = uni.GetTranslator("zh")
	v, ok := binding.Validator.Engine().(*val.Validate)
	if ok {
		_ = zhTranslations.RegisterDefaultTranslations(v, trans)
	}
}

func BindAndValid(c *gin.Context, target interface{}) bool {
	errs := make(map[string]string)
	err := c.ShouldBindJSON(target)
	if err != nil {
		log.Println("raw err", err)

		verrs, ok := err.(val.ValidationErrors)

		if !ok {
			log.Println("verrs", verrs)
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "请求参数错误",
				"code":    http.StatusNotAcceptable,
			})
			return false
		}

		for _, value := range verrs {
			t := reflect.ValueOf(target)
			realType := t.Type().Elem()
			field, _ := realType.FieldByName(value.StructField())
			errs[field.Tag.Get("json")] = value.Translate(trans)
		}

		c.JSON(http.StatusNotAcceptable, gin.H{
			"errors":  errs,
			"message": "请求参数错误",
			"code":    http.StatusNotAcceptable,
		})

		return false
	}

	return true
}
