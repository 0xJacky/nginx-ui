package api

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	val "github.com/go-playground/validator/v10"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"log"
	"net/http"
	"reflect"
	"regexp"
)

type JsonSnakeCase struct {
	Value interface{}
}

func (c JsonSnakeCase) MarshalJSON() ([]byte, error) {
	// Regexp definitions
	var keyMatchRegex = regexp.MustCompile(`\"(\w+)\":`)
	var wordBarrierRegex = regexp.MustCompile(`(\w)([A-Z])`)
	marshalled, err := json.Marshal(c.Value)
	converted := keyMatchRegex.ReplaceAllFunc(
		marshalled,
		func(match []byte) []byte {
			return bytes.ToLower(wordBarrierRegex.ReplaceAll(
				match,
				[]byte(`${1}_${2}`),
			))
		},
	)
	return converted, err
}

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

func BindAndValid(c *gin.Context, target interface{}) bool {
	errs := make(map[string]string)
	err := c.ShouldBindJSON(target)
	if err != nil {
		log.Println("raw err", err)
		uni := ut.New(zh.New())
		trans, _ := uni.GetTranslator("zh")
		v, ok := binding.Validator.Engine().(*val.Validate)
		if ok {
			_ = zhTranslations.RegisterDefaultTranslations(v, trans)
		}

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
