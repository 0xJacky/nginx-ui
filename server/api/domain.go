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
	"strings"
)

func GetDomains(c *gin.Context) {
	configFiles, err := ioutil.ReadDir(tool.GetNginxConfPath("sites-available"))

	if err != nil {
		log.Println(err)
	}

	enabledConfig, err := ioutil.ReadDir(filepath.Join(tool.GetNginxConfPath("sites-enabled")))

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	if err != nil {
		log.Println(err)
	}

	var configs []gin.H

	for i := range configFiles {
		file := configFiles[i]
		if !file.IsDir() {
			configs = append(configs, gin.H{
				"name":    file.Name(),
				"size":    file.Size(),
				"modify":  file.ModTime(),
				"enabled": enabledConfigMap[file.Name()],
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
	})
}

func GetDomain(c *gin.Context) {
	name := c.Param("name")
	path := filepath.Join(tool.GetNginxConfPath("sites-available"), name)

	enabled := true
	if _, err := os.Stat(filepath.Join(tool.GetNginxConfPath("sites-enabled"), name)); os.IsNotExist(err) {
		enabled = false
	}

	content, err := ioutil.ReadFile(path)

	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"message": err.Error(),
			})
		}
		log.Println(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
		"config": string(content),
	})

}

func AddDomain(c *gin.Context) {
	name := c.PostForm("name")
	SupportSSL := c.PostForm("support_ssl")

	baseKeys := []string{"http_listen_port",
		"https_listen_port",
		"server_name",
		"ssl_certificate", "ssl_certificate_key",
		"root", "extra",
	}

	tmp, err := ioutil.ReadFile("template/http-conf")

	log.Println(SupportSSL)
	if SupportSSL == "true" {
		tmp, err = ioutil.ReadFile("template/https-conf")
	}

	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "server error",
		})
		return
	}

	template := string(tmp)

	content := template

	for i := range baseKeys {
		content = strings.Replace(content, "{{ " + baseKeys[i] +" }}",
			c.PostForm(baseKeys[i]),
			-1)
	}

	log.Println(name, content)

	c.JSON(http.StatusOK, gin.H{
		"name": name,
		"content": content,
	})

}

func EditDomain(c *gin.Context) {
	name := c.Param("name")
	content := c.PostForm("content")
	path := filepath.Join(tool.GetNginxConfPath("sites-available"), name)

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

	if _, err := os.Stat(filepath.Join(tool.GetNginxConfPath("sites-enabled"), name)); os.IsExist(err) {
		tool.ReloadNginx()
	}

	GetDomain(c)
}

func EnableDomain(c *gin.Context) {
	configFilePath := filepath.Join(tool.GetNginxConfPath("sites-available"), c.Param("name"))
	enabledConfigFilePath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), c.Param("name"))

	_, err := os.Stat(configFilePath)

	if err != nil {
		log.Println(err)
	}

	err = os.Symlink(configFilePath, enabledConfigFilePath)

	if err != nil {
		log.Println(err)
	}

	tool.ReloadNginx()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DisableDomain(c *gin.Context) {
	enabledConfigFilePath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), c.Param("name"))

	_, err := os.Stat(enabledConfigFilePath)

	if err != nil {
		log.Println(err)
	}

	err = os.Remove(enabledConfigFilePath)

	if err != nil {
		log.Println(err)
	}

	tool.ReloadNginx()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteDomain(c *gin.Context)  {

}
