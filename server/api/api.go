package api

import (
    "github.com/gin-gonic/gin"
    "log"
    "net/http"

    ut "github.com/go-playground/universal-translator"
    val "github.com/go-playground/validator/v10"
)

func ErrorHandler(c *gin.Context, err error) {
    log.Println(err)
    c.JSON(http.StatusInternalServerError, gin.H{
        "message": err.Error(),
    })
}

type ValidError struct {
    Key     string
    Message string
}

type ValidErrors gin.H

func BindAndValid(c *gin.Context, v interface{}) (bool, ValidErrors) {
    errs := make(ValidErrors)
    err := c.ShouldBind(v)
    if err != nil {

        v := c.Value("trans")
        trans, _ := v.(ut.Translator)
        verrs, ok := err.(val.ValidationErrors)
        if !ok {
            return false, errs
        }

        for key, value := range verrs.Translate(trans) {
            errs[key] = value
        }

        return false, errs
    }

    return true, nil
}
