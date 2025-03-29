package streams

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
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
	SyncNodeIDs     []uint64                       `json:"sync_node_ids" gorm:"serializer:json"`
}

func GetStreams(c *gin.Context) {
	name := c.Query("name")
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")

	configFiles, err := os.ReadDir(nginx.GetConfPath("streams-available"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath("streams-enabled"))
	if err != nil {
		cosy.ErrHandler(c, err)
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
	name := c.Param("name")

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
		cosy.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if streamModel.Advanced {
		origContent, err := os.ReadFile(path)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		c.JSON(http.StatusOK, Stream{
			ModifiedAt:      file.ModTime(),
			Advanced:        streamModel.Advanced,
			Enabled:         enabled,
			Name:            name,
			Config:          string(origContent),
			ChatGPTMessages: chatgpt.Content,
			Filepath:        path,
			SyncNodeIDs:     streamModel.SyncNodeIDs,
		})
		return
	}

	nginxConfig, err := nginx.ParseNgxConfig(path)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, Stream{
		ModifiedAt:      file.ModTime(),
		Advanced:        streamModel.Advanced,
		Enabled:         enabled,
		Name:            name,
		Config:          nginxConfig.FmtCode(),
		Tokenized:       nginxConfig,
		ChatGPTMessages: chatgpt.Content,
		Filepath:        path,
		SyncNodeIDs:     streamModel.SyncNodeIDs,
	})
}

func SaveStream(c *gin.Context) {
	name := c.Param("name")

	var json struct {
		Content     string   `json:"content" binding:"required"`
		SyncNodeIDs []uint64 `json:"sync_node_ids"`
		Overwrite   bool     `json:"overwrite"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := stream.Save(name, json.Content, json.Overwrite, json.SyncNodeIDs)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	GetStream(c)
}

func EnableStream(c *gin.Context) {
	err := stream.Enable(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DisableStream(c *gin.Context) {
	err := stream.Disable(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteStream(c *gin.Context) {
	err := stream.Delete(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func RenameStream(c *gin.Context) {
	oldName := c.Param("name")
	var json struct {
		NewName string `json:"new_name"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := stream.Rename(oldName, json.NewName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
