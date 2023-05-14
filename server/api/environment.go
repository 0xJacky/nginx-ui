package api

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/0xJacky/Nginx-UI/server/service"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
)

func GetEnvironment(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	envQuery := query.Environment

	environment, err := envQuery.FirstByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, environment)
}

func GetEnvironmentList(c *gin.Context) {
	data, err := service.RetrieveEnvironmentList()
	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

type EnvironmentManageJson struct {
	Name  string `json:"name" binding:"required"`
	URL   string `json:"url" binding:"required"`
	Token string `json:"token"  binding:"required"`
}

func AddEnvironment(c *gin.Context) {
	var json EnvironmentManageJson
	if !BindAndValid(c, &json) {
		return
	}

	environment := model.Environment{
		Name:  json.Name,
		URL:   json.URL,
		Token: json.Token,
	}

	envQuery := query.Environment

	err := envQuery.Create(&environment)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, environment)
}

func EditEnvironment(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	var json EnvironmentManageJson
	if !BindAndValid(c, &json) {
		return
	}

	envQuery := query.Environment

	environment, err := envQuery.FirstByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	_, err = envQuery.Where(envQuery.ID.Eq(environment.ID)).Updates(&model.Environment{
		Name:  json.Name,
		URL:   json.URL,
		Token: json.Token,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	GetEnvironment(c)
}

func DeleteEnvironment(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))
	envQuery := query.Environment

	environment, err := envQuery.FirstByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	err = envQuery.DeleteByID(environment.ID)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
