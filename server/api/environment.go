package api

import (
	"github.com/0xJacky/Nginx-UI/server/internal/analytic"
	"github.com/0xJacky/Nginx-UI/server/internal/environment"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
	"regexp"
)

func GetEnvironment(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	envQuery := query.Environment

	env, err := envQuery.FirstByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, env)
}

func GetEnvironmentList(c *gin.Context) {
	data, err := environment.RetrieveEnvironmentList()
	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

type EnvironmentManageJson struct {
	Name          string `json:"name" binding:"required"`
	URL           string `json:"url" binding:"required"`
	Token         string `json:"token"  binding:"required"`
	OperationSync bool   `json:"operation_sync"`
	SyncApiRegex  string `json:"sync_api_regex"`
}

func validateRegex(data EnvironmentManageJson) error {
	if data.OperationSync {
		_, err := regexp.Compile(data.SyncApiRegex)
		return err
	}
	return nil
}

func AddEnvironment(c *gin.Context) {
	var json EnvironmentManageJson
	if !BindAndValid(c, &json) {
		return
	}
	if err := validateRegex(json); err != nil {
		ErrHandler(c, err)
		return
	}

	env := model.Environment{
		Name:          json.Name,
		URL:           json.URL,
		Token:         json.Token,
		OperationSync: &json.OperationSync,
		SyncApiRegex:  json.SyncApiRegex,
	}

	envQuery := query.Environment

	err := envQuery.Create(&env)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	go analytic.RestartRetrieveNodesStatus()

	c.JSON(http.StatusOK, env)
}

func EditEnvironment(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	var json EnvironmentManageJson
	if !BindAndValid(c, &json) {
		return
	}
	if err := validateRegex(json); err != nil {
		ErrHandler(c, err)
		return
	}

	envQuery := query.Environment

	env, err := envQuery.FirstByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	_, err = envQuery.Where(envQuery.ID.Eq(env.ID)).Updates(&model.Environment{
		Name:          json.Name,
		URL:           json.URL,
		Token:         json.Token,
		OperationSync: &json.OperationSync,
		SyncApiRegex:  json.SyncApiRegex,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	go analytic.RestartRetrieveNodesStatus()

	GetEnvironment(c)
}

func DeleteEnvironment(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))
	envQuery := query.Environment

	env, err := envQuery.FirstByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	err = envQuery.DeleteByID(env.ID)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	go analytic.RestartRetrieveNodesStatus()

	c.JSON(http.StatusNoContent, nil)
}
