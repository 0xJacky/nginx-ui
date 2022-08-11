package api

import (
	"github.com/0xJacky/Nginx-UI/server/pkg/config_list"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GetConfigs(c *gin.Context) {
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")

	mySort := map[string]string{
		"name":   "string",
		"modify": "time",
	}

	configFiles, err := os.ReadDir(nginx.GetNginxConfPath("/"))

	if err != nil {
		ErrHandler(c, err)
		return
	}

	var configs []gin.H

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()

		if !file.IsDir() && "." != file.Name()[0:1] {
			configs = append(configs, gin.H{
				"name":   file.Name(),
				"size":   fileInfo.Size(),
				"modify": fileInfo.ModTime(),
			})
		}
	}

	configs = config_list.Sort(orderBy, sort, mySort[orderBy], configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}

func GetConfig(c *gin.Context) {
	name := c.Param("name")
	path := filepath.Join(nginx.GetNginxConfPath("/"), name)

	content, err := os.ReadFile(path)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": string(content),
	})

}

type AddConfigJson struct {
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"`
}

func AddConfig(c *gin.Context) {
	var request AddConfigJson
	err := c.BindJSON(&request)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	name := request.Name
	content := request.Content

	path := filepath.Join(nginx.GetNginxConfPath("/"), name)

	log.Println(path)
	if _, err = os.Stat(path); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "config exist",
		})
		return
	}

	if content != "" {
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	output := nginx.ReloadNginx()

	if output != "" && strings.Contains(output, "error") {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    name,
		"content": content,
	})

}

type EditConfigJson struct {
	Content string `json:"content" binding:"required"`
}

func EditConfig(c *gin.Context) {
	name := c.Param("name")
	var request EditConfigJson
	err := c.BindJSON(&request)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	path := filepath.Join(nginx.GetNginxConfPath("/"), name)
	content := request.Content

	origContent, err := os.ReadFile(path)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	if content != "" && content != string(origContent) {
		// model.CreateBackup(path)
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	output := nginx.ReloadNginx()

	if output != "" && strings.Contains(output, "error") {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	GetConfig(c)
}
