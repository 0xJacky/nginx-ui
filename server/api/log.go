package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/gin-gonic/gin"
)

func ListLog(c *gin.Context) {
	c.JSON(http.StatusOK, model.ListLog(c))
	return
}

func CreateLog(c *gin.Context) {
	var l model.Log
	if ok := BindAndValid(c, &l); !ok {
		c.JSON(http.StatusBadRequest, errors.New("invalid request body"))
		return
	}
	if err := model.CreateLog(l); err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, errors.New("internal error"))
		return
	}
	c.JSON(http.StatusOK, l)
	return
}

func Deletelog(c *gin.Context) {
	id := c.Param("id")
	err := model.DeleteLog(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "internal error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

func GetLog(c *gin.Context) {
	id := c.Param("id")
	log, err := model.GetLogByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, log)
}

type LogJson struct {
	ID   string `json:"id" binding:"required,max=255"`
	Path string `json:"path" binding:"max=255"`
}

func EditLog(c *gin.Context) {
	id := c.Param("id")
	var json model.Log
	ok := BindAndValid(c, &json)
	if !ok {
		return
	}

	old, err := model.GetLogByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	if err := model.EditLog(old, json); err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, json)
}
