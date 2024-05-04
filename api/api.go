package api

import (
	"errors"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
	"reflect"
	"regexp"
	"strings"
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
	err := c.ShouldBindJSON(target)
	if err != nil {
		logger.Error("bind err", err)

		var verrs validator.ValidationErrors
		ok := errors.As(err, &verrs)

		if !ok {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "Requested with wrong parameters",
				"code":    http.StatusNotAcceptable,
			})
			return false
		}

		t := reflect.TypeOf(target).Elem()
		errorsMap := make(map[string]interface{})
		for _, value := range verrs {
			var path []string

			namespace := strings.Split(value.StructNamespace(), ".")
			// logger.Debug(t.Name(), namespace)
			if t.Name() != "" && len(namespace) > 1 {
				namespace = namespace[1:]
			}

			getJsonPath(t, namespace, &path)
			insertError(errorsMap, path, value.Tag())
		}

		c.JSON(http.StatusNotAcceptable, gin.H{
			"errors":  errorsMap,
			"message": "Requested with wrong parameters",
			"code":    http.StatusNotAcceptable,
		})

		return false
	}

	return true
}

// findField recursively finds the field in a nested struct
func getJsonPath(t reflect.Type, fields []string, path *[]string) {
	field := fields[0]
	// used in case of array
	var index string
	if field[len(field)-1] == ']' {
		re := regexp.MustCompile(`(\w+)\[(\d+)\]`)
		matches := re.FindStringSubmatch(field)

		if len(matches) > 2 {
			field = matches[1]
			index = matches[2]
		}
	}

	f, ok := t.FieldByName(field)
	if !ok {
		return
	}

	*path = append(*path, f.Tag.Get("json"))

	if index != "" {
		*path = append(*path, index)
	}

	if len(fields) > 1 {
		subFields := fields[1:]
		getJsonPath(f.Type, subFields, path)
	}
}

// insertError inserts an error into the errors map
func insertError(errorsMap map[string]interface{}, path []string, errorTag string) {
	if len(path) == 0 {
		return
	}

	jsonTag := path[0]
	if len(path) == 1 {
		// Last element in the path, set the error
		errorsMap[jsonTag] = errorTag
		return
	}

	// Create a new map if necessary
	if _, ok := errorsMap[jsonTag]; !ok {
		errorsMap[jsonTag] = make(map[string]interface{})
	}

	// Recursively insert into the nested map
	subMap, _ := errorsMap[jsonTag].(map[string]interface{})
	insertError(subMap, path[1:], errorTag)
}
