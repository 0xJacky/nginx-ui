package api

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/tool"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
)

func GetConfigs(c *gin.Context) {
	configFiles, err := ioutil.ReadDir(filepath.Join("/usr/local/etc/nginx"))

	if err != nil {
		log.Println(err)
	}

	var configs []gin.H

	for i := range configFiles {
		file := configFiles[i]

		if !file.IsDir() && "." != file.Name()[0:1] {
			configs = append(configs, gin.H{
				"name":    file.Name(),
				"size":    file.Size(),
				"modify":  file.ModTime(),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
	})
}

func GetConfig(c *gin.Context) {
	name := c.Param("name")
	path := filepath.Join("/usr/local/etc/nginx", name)

	content, err := ioutil.ReadFile(path)

	if err != nil {
		log.Println(err)
	}

	c.JSON(http.StatusOK, gin.H{
		"config": string(content),
	})

}

func AddConfig(c *gin.Context) {
	name := c.PostForm("name")
	content := c.PostForm("content")
	path := filepath.Join(tool.GetNginxConfPath("/"), name)

	s, err := strconv.Unquote(`"` + content + `"`)
	if err != nil {
		log.Println(err)
	}

	if s != "" {
		err := ioutil.WriteFile(path, []byte(s), 0644)
		if err != nil {
			log.Println(err)
		}
	}

	tool.ReloadNginx()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})

}

func EditConfig(c *gin.Context) {
	name := c.Param("name")
	content := c.PostForm("content")
	path := filepath.Join(tool.GetNginxConfPath("/"), name)

	s, err := strconv.Unquote(`"` + content + `"`)
	if err != nil {
		log.Println(err)
	}

	origContent, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println(err)
	}

	if s != "" && s != string(origContent) {
		model.CreateBackup(path)
		err := ioutil.WriteFile(path, []byte(s), 0644)
		if err != nil {
			log.Println(err)
		}
	}

	tool.ReloadNginx()

	GetConfig(c)
}
