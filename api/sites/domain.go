package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/uozi-tech/cosy/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"os"
)

func GetSite(c *gin.Context) {
	rewriteName, ok := c.Get("rewriteConfigFileName")
	name := c.Param("name")

	// for modify filename
	if ok {
		name = rewriteName.(string)
	}

	path := nginx.GetConfPath("sites-available", name)
	file, err := os.Stat(path)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	enabled := true
	if _, err := os.Stat(nginx.GetConfPath("sites-enabled", name)); os.IsNotExist(err) {
		enabled = false
	}

	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(path)).FirstOrCreate()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	s := query.Site
	site, err := s.Where(s.Path.Eq(path)).FirstOrInit()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	certModel, err := model.FirstCert(name)
	if err != nil {
		logger.Warn(err)
	}

	if site.Advanced {
		origContent, err := os.ReadFile(path)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}

		c.JSON(http.StatusOK, Site{
			ModifiedAt:      file.ModTime(),
			Advanced:        site.Advanced,
			Enabled:         enabled,
			Name:            name,
			Config:          string(origContent),
			AutoCert:        certModel.AutoCert == model.AutoCertEnabled,
			ChatGPTMessages: chatgpt.Content,
			Filepath:        path,
		})
		return
	}

	nginxConfig, err := nginx.ParseNgxConfig(path)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	certInfoMap := make(map[int][]*cert.Info)
	for serverIdx, server := range nginxConfig.Servers {
		for _, directive := range server.Directives {
			if directive.Directive == "ssl_certificate" {
				pubKey, err := cert.GetCertInfo(directive.Params)
				if err != nil {
					logger.Error("Failed to get certificate information", err)
					continue
				}
				certInfoMap[serverIdx] = append(certInfoMap[serverIdx], pubKey)
			}
		}
	}

	c.JSON(http.StatusOK, Site{
		ModifiedAt:      file.ModTime(),
		Advanced:        site.Advanced,
		Enabled:         enabled,
		Name:            name,
		Config:          nginxConfig.FmtCode(),
		Tokenized:       nginxConfig,
		AutoCert:        certModel.AutoCert == model.AutoCertEnabled,
		CertInfo:        certInfoMap,
		ChatGPTMessages: chatgpt.Content,
		Filepath:        path,
	})
}

func SaveSite(c *gin.Context) {
	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "param name is empty",
		})
		return
	}

	var json struct {
		Name      string `json:"name" binding:"required"`
		Content   string `json:"content" binding:"required"`
		Overwrite bool   `json:"overwrite"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	path := nginx.GetConfPath("sites-available", name)

	if !json.Overwrite && helper.FileExists(path) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	err := os.WriteFile(path, []byte(json.Content), 0644)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)
	// rename the config file if needed
	if name != json.Name {
		newPath := nginx.GetConfPath("sites-available", json.Name)
		s := query.Site
		_, err = s.Where(s.Path.Eq(path)).Update(s.Path, newPath)

		// check if dst file exists, do not rename
		if helper.FileExists(newPath) {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "File exists",
			})
			return
		}
		// recreate a soft link
		if helper.FileExists(enabledConfigFilePath) {
			_ = os.Remove(enabledConfigFilePath)
			enabledConfigFilePath = nginx.GetConfPath("sites-enabled", json.Name)
			err = os.Symlink(newPath, enabledConfigFilePath)

			if err != nil {
				api.ErrHandler(c, err)
				return
			}
		}

		err = os.Rename(path, newPath)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}

		name = json.Name
		c.Set("rewriteConfigFileName", name)
	}

	enabledConfigFilePath = nginx.GetConfPath("sites-enabled", name)
	if helper.FileExists(enabledConfigFilePath) {
		// Test nginx configuration
		output := nginx.TestConf()

		if nginx.GetLogLevel(output) > nginx.Warn {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": output,
			})
			return
		}

		output = nginx.Reload()

		if nginx.GetLogLevel(output) > nginx.Warn {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": output,
			})
			return
		}
	}

	GetSite(c)
}

func EnableSite(c *gin.Context) {
	configFilePath := nginx.GetConfPath("sites-available", c.Param("name"))
	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", c.Param("name"))

	_, err := os.Stat(configFilePath)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if _, err = os.Stat(enabledConfigFilePath); os.IsNotExist(err) {
		err = os.Symlink(configFilePath, enabledConfigFilePath)

		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	// Test nginx config, if not pass, then disable the site.
	output := nginx.TestConf()
	if nginx.GetLogLevel(output) > nginx.Warn {
		_ = os.Remove(enabledConfigFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	output = nginx.Reload()

	if nginx.GetLogLevel(output) > nginx.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DisableSite(c *gin.Context) {
	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", c.Param("name"))
	_, err := os.Stat(enabledConfigFilePath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = os.Remove(enabledConfigFilePath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	// delete auto cert record
	certModel := model.Cert{Filename: c.Param("name")}
	err = certModel.Remove()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	output := nginx.Reload()
	if nginx.GetLogLevel(output) > nginx.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteSite(c *gin.Context) {
	var err error
	name := c.Param("name")
	availablePath := nginx.GetConfPath("sites-available", name)
	enabledPath := nginx.GetConfPath("sites-enabled", name)
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

	certModel := model.Cert{Filename: name}
	_ = certModel.Remove()

	err = os.Remove(availablePath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
