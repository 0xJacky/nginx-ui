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
	"strings"
)

func GetDomains(c *gin.Context) {
	configFiles, err := ioutil.ReadDir(tool.GetNginxConfPath("sites-available"))

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	enabledConfig, err := ioutil.ReadDir(filepath.Join(tool.GetNginxConfPath("sites-enabled")))

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	if err != nil {
		ErrorHandler(c, err)
		return
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
		ErrorHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
		"config": string(content),
	})

}

func AddDomain(c *gin.Context) {
	request := make(gin.H)
	err := c.BindJSON(&request)
	if err != nil {
		ErrorHandler(c, err)
		return
	}

	name := request["name"].(string)
	path := filepath.Join(tool.GetNginxConfPath("sites-available"), name)
	log.Println(path)
	if _, err = os.Stat(path); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "site exist",
		})
		return
	}

	SupportSSL := request["support_ssl"]

	baseKeys := []string{"http_listen_port",
		"https_listen_port",
		"server_name",
		"ssl_certificate", "ssl_certificate_key",
		"index", "root", "extra",
	}

	tmp, err := ioutil.ReadFile("template/http-conf")

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	if SupportSSL == true {
		tmp, err = ioutil.ReadFile("template/https-conf")
	}

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	template := string(tmp)

	content := template

	for i := range baseKeys {
		val, ok := request[baseKeys[i]]
		replace := ""
		if ok {
			replace = val.(string)
		}
		content = strings.Replace(content, "{{ "+baseKeys[i]+" }}",
			replace, -1)
	}

	log.Println(name, content)

	err = ioutil.WriteFile(path, []byte(content), 0644)

	if err != nil {
		ErrorHandler(c, err)
		return
	}


	c.JSON(http.StatusOK, gin.H{
		"name": name,
		"content": content,
	})

}

func EditDomain(c *gin.Context) {
	var err error
	var origContent []byte
	name := c.Param("name")
	request := make(gin.H)
	err = c.BindJSON(&request)
	path := filepath.Join(tool.GetNginxConfPath("sites-available"), name)

	if _, err = os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "site not found",
		})
		return
	}

	origContent, err = ioutil.ReadFile(path)
	if err != nil {
		ErrorHandler(c, err)
		return
	}

	if request["content"] != "" && request["content"] != string(origContent) {
		model.CreateBackup(path)
		err := ioutil.WriteFile(path, []byte(request["content"].(string)), 0644)
		if err != nil {
			ErrorHandler(c, err)
			return
		}
	}

	if _, err := os.Stat(filepath.Join(tool.GetNginxConfPath("sites-enabled"), name)); err == nil {
		tool.ReloadNginx()
	}

	GetDomain(c)
}

func EnableDomain(c *gin.Context) {
	configFilePath := filepath.Join(tool.GetNginxConfPath("sites-available"), c.Param("name"))
	enabledConfigFilePath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), c.Param("name"))

	_, err := os.Stat(configFilePath)

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	err = os.Symlink(configFilePath, enabledConfigFilePath)

	if err != nil {
		ErrorHandler(c, err)
		return
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
		ErrorHandler(c, err)
		return
	}

	err = os.Remove(enabledConfigFilePath)

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	tool.ReloadNginx()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteDomain(c *gin.Context)  {
	var err error
	name := c.Param("name")
	request := make(gin.H)
	err = c.BindJSON(request)
	availablePath := filepath.Join(tool.GetNginxConfPath("sites-available"), name)
	enabledPath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), name)

	if _, err = os.Stat(availablePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "site not found",
		})
		return
	}

	if _, err = os.Stat(enabledPath); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "site is enabled",
		})
		return
	}

	err = os.Remove(availablePath)

	if err != nil {
		ErrorHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})

}
