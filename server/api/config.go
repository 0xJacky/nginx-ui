package api

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/tool"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

func GetConfigs(c *gin.Context) {
	configFiles, err := ioutil.ReadDir(tool.GetNginxConfPath("/"))

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	var configs []gin.H

	for i := range configFiles {
		file := configFiles[i]

		if !file.IsDir() && "." != file.Name()[0:1] {
			configs = append(configs, gin.H{
				"name":   file.Name(),
				"size":   file.Size(),
				"modify": file.ModTime(),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
	})
}

func GetConfig(c *gin.Context) {
	name := c.Param("name")
	path := filepath.Join(tool.GetNginxConfPath("/"), name)

	content, err := ioutil.ReadFile(path)

	if err != nil {
		ErrorHandler(c, err)
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
		ErrorHandler(c, err)
		return
	}

	name := request.Name
	content := request.Content

	path := filepath.Join(tool.GetNginxConfPath("/"), name)

	log.Println(path)
	if _, err = os.Stat(path); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "config exist",
		})
		return
	}

	if content != "" {
		err := ioutil.WriteFile(path, []byte(content), 0644)
		if err != nil {
			ErrorHandler(c, err)
			return
		}
	}

	tool.ReloadNginx()

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
		ErrorHandler(c, err)
		return
	}
	path := filepath.Join(tool.GetNginxConfPath("/"), name)
	content := request.Content

	s, err := strconv.Unquote(`"` + content + `"`)
	if err != nil {
		ErrorHandler(c, err)
		return
	}

	origContent, err := ioutil.ReadFile(path)
	if err != nil {
		ErrorHandler(c, err)
		return
	}

	if s != "" && s != string(origContent) {
		model.CreateBackup(path)
		err := ioutil.WriteFile(path, []byte(s), 0644)
		if err != nil {
			ErrorHandler(c, err)
			return
		}
	}

	tool.ReloadNginx()

	GetConfig(c)
}
