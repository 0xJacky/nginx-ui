package streams

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm/clause"
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
	EnvGroupID      uint64                         `json:"env_group_id"`
	EnvGroup        *model.EnvGroup                `json:"env_group,omitempty"`
	SyncNodeIDs     []uint64                       `json:"sync_node_ids" gorm:"serializer:json"`
}

func GetStreams(c *gin.Context) {
	name := c.Query("name")
	status := c.Query("status")
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")
	queryEnvGroupId := cast.ToUint64(c.Query("env_group_id"))

	configFiles, err := os.ReadDir(nginx.GetConfPath("streams-available"))
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(stream.ErrReadDirFailed, err.Error()))
		return
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath("streams-enabled"))
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(stream.ErrReadDirFailed, err.Error()))
		return
	}

	enabledConfigMap := make(map[string]config.ConfigStatus)
	for _, file := range configFiles {
		enabledConfigMap[file.Name()] = config.StatusDisabled
	}
	for i := range enabledConfig {
		enabledConfigMap[nginx.GetConfNameBySymlinkName(enabledConfig[i].Name())] = config.StatusEnabled
	}

	var configs []config.Config

	// Get all streams map for Node Group lookup
	s := query.Stream
	var streams []*model.Stream
	if queryEnvGroupId != 0 {
		streams, err = s.Where(s.EnvGroupID.Eq(queryEnvGroupId)).Find()
	} else {
		streams, err = s.Find()
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Retrieve Node Groups data
	eg := query.EnvGroup
	envGroups, err := eg.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// Create a map of Node Groups for quick lookup by ID
	envGroupMap := lo.SliceToMap(envGroups, func(item *model.EnvGroup) (uint64, *model.EnvGroup) {
		return item.ID, item
	})

	// Convert streams slice to map for efficient lookups
	streamsMap := lo.SliceToMap(streams, func(item *model.Stream) (string, *model.Stream) {
		// Associate each stream with its corresponding Node Group
		if item.EnvGroupID > 0 {
			item.EnvGroup = envGroupMap[item.EnvGroupID]
		}
		return filepath.Base(item.Path), item
	})

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()
		if file.IsDir() {
			continue
		}

		// Apply name filter if specified
		if name != "" && !strings.Contains(file.Name(), name) {
			continue
		}

		// Apply enabled status filter if specified
		if status != "" && enabledConfigMap[file.Name()] != config.ConfigStatus(status) {
			continue
		}

		var (
			envGroupId uint64
			envGroup   *model.EnvGroup
		)

		// Lookup stream in the streams map to get Node Group info
		if stream, ok := streamsMap[file.Name()]; ok {
			envGroupId = stream.EnvGroupID
			envGroup = stream.EnvGroup
		}

		// Apply Node Group filter if specified
		if queryEnvGroupId != 0 && envGroupId != queryEnvGroupId {
			continue
		}

		// Add the config to the result list after passing all filters
		configs = append(configs, config.Config{
			Name:       file.Name(),
			ModifiedAt: fileInfo.ModTime(),
			Size:       fileInfo.Size(),
			IsDir:      fileInfo.IsDir(),
			Status:     enabledConfigMap[file.Name()],
			EnvGroupID: envGroupId,
			EnvGroup:   envGroup,
		})
	}

	// Sort the configs based on the provided sort parameters
	configs = config.Sort(orderBy, sort, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}

func GetStream(c *gin.Context) {
	name := c.Param("name")

	// Get the absolute path to the stream configuration file
	path := nginx.GetConfPath("streams-available", name)
	file, err := os.Stat(path)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	// Check if the stream is enabled
	enabled := true
	if _, err := os.Stat(nginx.GetConfPath("streams-enabled", name)); os.IsNotExist(err) {
		enabled = false
	}

	// Retrieve or create ChatGPT log for this stream
	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Initialize empty content if nil
	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	// Retrieve or create stream model from database
	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// For advanced mode, return the raw content
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
			EnvGroupID:      streamModel.EnvGroupID,
			EnvGroup:        streamModel.EnvGroup,
			SyncNodeIDs:     streamModel.SyncNodeIDs,
		})
		return
	}

	// For normal mode, parse and tokenize the configuration
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
		EnvGroupID:      streamModel.EnvGroupID,
		EnvGroup:        streamModel.EnvGroup,
		SyncNodeIDs:     streamModel.SyncNodeIDs,
	})
}

func SaveStream(c *gin.Context) {
	name := c.Param("name")

	var json struct {
		Content     string   `json:"content" binding:"required"`
		EnvGroupID  uint64   `json:"env_group_id"`
		SyncNodeIDs []uint64 `json:"sync_node_ids"`
		Overwrite   bool     `json:"overwrite"`
		PostAction  string   `json:"post_action"`
	}

	// Validate input JSON
	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Get stream from database or create if not exists
	path := nginx.GetConfPath("streams-available", name)
	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Update Node Group ID if provided
	if json.EnvGroupID > 0 {
		streamModel.EnvGroupID = json.EnvGroupID
	}

	// Update synchronization node IDs if provided
	if json.SyncNodeIDs != nil {
		streamModel.SyncNodeIDs = json.SyncNodeIDs
	}

	// Save the updated stream model to database
	_, err = s.Where(s.ID.Eq(streamModel.ID)).Updates(streamModel)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Save the stream configuration file
	err = stream.Save(name, json.Content, json.Overwrite, json.SyncNodeIDs, json.PostAction)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Return the updated stream
	GetStream(c)
}

func EnableStream(c *gin.Context) {
	// Enable the stream by creating a symlink in streams-enabled directory
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
	// Disable the stream by removing the symlink from streams-enabled directory
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
	// Delete the stream configuration file and its symbolic link if exists
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
	// Validate input JSON
	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Rename the stream configuration file
	err := stream.Rename(oldName, json.NewName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func BatchUpdateStreams(c *gin.Context) {
	cosy.Core[model.Stream](c).SetValidRules(gin.H{
		"env_group_id": "required",
	}).SetItemKey("path").
		BeforeExecuteHook(func(ctx *cosy.Ctx[model.Stream]) {
			effectedPath := make([]string, len(ctx.BatchEffectedIDs))
			var streams []*model.Stream
			for i, name := range ctx.BatchEffectedIDs {
				path := nginx.GetConfPath("streams-available", name)
				effectedPath[i] = path
				streams = append(streams, &model.Stream{
					Path: path,
				})
			}
			s := query.Stream
			err := s.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(streams...)
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			ctx.BatchEffectedIDs = effectedPath
		}).BatchModify()
}
