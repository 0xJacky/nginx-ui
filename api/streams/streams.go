package streams

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"os"
	"strings"
	"time"
)

type Stream struct {
	ModifiedAt      time.Time                      `json:"modified_at"`
	Advanced        bool                           `json:"advanced"`
	Enabled         bool                           `json:"enabled"`
	Name            string                         `json:"name"`
	Config          string                         `json:"config"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	Tokenized       *nginx.NgxConfig               `json:"tokenized,omitempty"`
	Filepath        string                         `json:"filepath"`
}

func GetStreams(c *gin.Context) {
	name := c.Query("name")
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")

	configFiles, err := os.ReadDir(nginx.GetConfPath("streams-available"))

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath("streams-enabled"))

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	var configs []config.Config

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()
		if !file.IsDir() {
			if name != "" && !strings.Contains(file.Name(), name) {
				continue
			}
			configs = append(configs, config.Config{
				Name:       file.Name(),
				ModifiedAt: fileInfo.ModTime(),
				Size:       fileInfo.Size(),
				IsDir:      fileInfo.IsDir(),
				Enabled:    enabledConfigMap[file.Name()],
			})
		}
	}

	configs = config.Sort(orderBy, sort, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}

func GetStream(c *gin.Context) {
	rewriteName, ok := c.Get("rewriteConfigFileName")

	name := c.Param("name")

	// for modify filename
	if ok {
		name = rewriteName.(string)
	}

	path := nginx.GetConfPath("streams-available", name)
	file, err := os.Stat(path)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	enabled := true

	if _, err := os.Stat(nginx.GetConfPath("streams-enabled", name)); os.IsNotExist(err) {
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

	s := query.Stream
	stream, err := s.Where(s.Path.Eq(path)).FirstOrInit()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if stream.Advanced {
		origContent, err := os.ReadFile(path)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}

		c.JSON(http.StatusOK, Stream{
			ModifiedAt:      file.ModTime(),
			Advanced:        stream.Advanced,
			Enabled:         enabled,
			Name:            name,
			Config:          string(origContent),
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

	c.JSON(http.StatusOK, Stream{
		ModifiedAt:      file.ModTime(),
		Advanced:        stream.Advanced,
		Enabled:         enabled,
		Name:            name,
		Config:          nginxConfig.FmtCode(),
		Tokenized:       nginxConfig,
		ChatGPTMessages: chatgpt.Content,
		Filepath:        path,
	})
}

func SaveStream(c *gin.Context) {
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

	path := nginx.GetConfPath("streams-available", name)

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
	enabledConfigFilePath := nginx.GetConfPath("streams-enabled", name)
	// rename the config file if needed
	if name != json.Name {
		newPath := nginx.GetConfPath("streams-available", json.Name)
		s := query.Stream
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
			enabledConfigFilePath = nginx.GetConfPath("streams-enabled", json.Name)
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

	enabledConfigFilePath = nginx.GetConfPath("streams-enabled", name)
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

	GetStream(c)
}

func EnableStream(c *gin.Context) {
	configFilePath := nginx.GetConfPath("streams-available", c.Param("name"))
	enabledConfigFilePath := nginx.GetConfPath("streams-enabled", c.Param("name"))

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

	// Test nginx config, if not pass, then disable the stream.
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

func DisableStream(c *gin.Context) {
	enabledConfigFilePath := nginx.GetConfPath("streams-enabled", c.Param("name"))

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

func DeleteStream(c *gin.Context) {
	var err error
	name := c.Param("name")
	availablePath := nginx.GetConfPath("streams-available", name)
	enabledPath := nginx.GetConfPath("streams-enabled", name)

	if _, err = os.Stat(availablePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "stream not found",
		})
		return
	}

	if _, err = os.Stat(enabledPath); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "stream is enabled",
		})
		return
	}

	if err = os.Remove(availablePath); err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
