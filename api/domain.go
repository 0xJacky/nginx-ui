package api

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/tool"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func GetDomains(c *gin.Context) {
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")

	mySort := map[string]string{
		"enabled": "bool",
		"name":    "string",
		"modify":  "time",
	}

	configFiles, err := ioutil.ReadDir(tool.GetNginxConfPath("sites-available"))

	if err != nil {
		ErrHandler(c, err)
		return
	}

	enabledConfig, err := ioutil.ReadDir(filepath.Join(tool.GetNginxConfPath("sites-enabled")))

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	if err != nil {
		ErrHandler(c, err)
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

	configs = tool.Sort(orderBy, sort, mySort[orderBy], configs)

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
			return
		}
		ErrHandler(c, err)
		return
	}

	_, err = model.FirstCert(name)

	c.JSON(http.StatusOK, gin.H{
		"enabled":   enabled,
		"name":      name,
		"config":    string(content),
		"auto_cert": err == nil,
	})

}

func EditDomain(c *gin.Context) {
	var err error
	name := c.Param("name")
	request := make(gin.H)
	err = c.BindJSON(&request)
	path := filepath.Join(tool.GetNginxConfPath("sites-available"), name)

	err = ioutil.WriteFile(path, []byte(request["content"].(string)), 0644)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	enabledConfigFilePath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), name)
	if _, err = os.Stat(enabledConfigFilePath); err == nil {
		// 测试配置文件
		err = tool.TestNginxConf(enabledConfigFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		output := tool.ReloadNginx()

		if output != "" {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": output,
			})
			return
		}
	}

	GetDomain(c)
}

func EnableDomain(c *gin.Context) {
	configFilePath := filepath.Join(tool.GetNginxConfPath("sites-available"), c.Param("name"))
	enabledConfigFilePath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), c.Param("name"))

	_, err := os.Stat(configFilePath)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = os.Symlink(configFilePath, enabledConfigFilePath)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	// 测试配置文件，不通过则撤回启用
	err = tool.TestNginxConf(enabledConfigFilePath)
	if err != nil {
		_ = os.Remove(enabledConfigFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	output := tool.ReloadNginx()

	if output != "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DisableDomain(c *gin.Context) {
	enabledConfigFilePath := filepath.Join(tool.GetNginxConfPath("sites-enabled"), c.Param("name"))

	_, err := os.Stat(enabledConfigFilePath)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = os.Remove(enabledConfigFilePath)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	output := tool.ReloadNginx()

	if output != "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteDomain(c *gin.Context) {
	var err error
	name := c.Param("name")
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

	cert := model.Cert{Domain: name}
	_ = cert.Remove()

	err = os.Remove(availablePath)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})

}

func AddDomainToAutoCert(c *gin.Context) {
	domain := c.Param("domain")
	cert, err := model.FirstOrCreateCert(domain)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, cert)
}

func RemoveDomainFromAutoCert(c *gin.Context) {
	cert := model.Cert{
		Domain: c.Param("domain"),
	}
	err := cert.Remove()

	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, nil)
}
