package streams

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm/clause"
)

type Stream struct {
	ModifiedAt   time.Time            `json:"modified_at"`
	Advanced     bool                 `json:"advanced"`
	Status       config.ConfigStatus  `json:"status"`
	Name         string               `json:"name"`
	Config       string               `json:"config"`
	Tokenized    *nginx.NgxConfig     `json:"tokenized,omitempty"`
	Filepath     string               `json:"filepath"`
	EnvGroupID   uint64               `json:"env_group_id"`
	EnvGroup     *model.EnvGroup      `json:"env_group,omitempty"`
	SyncNodeIDs  []uint64             `json:"sync_node_ids" gorm:"serializer:json"`
	ProxyTargets []config.ProxyTarget `json:"proxy_targets,omitempty"`
}

func GetStreams(c *gin.Context) {
	// Parse query parameters
	options := &stream.ListOptions{
		Search:     c.Query("search"),
		Name:       c.Query("name"),
		Status:     c.Query("status"),
		OrderBy:    c.Query("order_by"),
		Sort:       c.DefaultQuery("sort", "desc"),
		EnvGroupID: cast.ToUint64(c.Query("env_group_id")),
	}

	// Get streams from database
	s := query.Stream
	eg := query.EnvGroup

	// Get environment groups for association
	envGroups, err := eg.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Create environment group map for quick lookup
	envGroupMap := lo.SliceToMap(envGroups, func(item *model.EnvGroup) (uint64, *model.EnvGroup) {
		return item.ID, item
	})

	// Get streams with optional filtering
	var streams []*model.Stream
	if options.EnvGroupID != 0 {
		streams, err = s.Where(s.EnvGroupID.Eq(options.EnvGroupID)).Find()
	} else {
		streams, err = s.Find()
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Associate streams with their environment groups
	for _, stream := range streams {
		if stream.EnvGroupID > 0 {
			stream.EnvGroup = envGroupMap[stream.EnvGroupID]
		}
	}

	// Get stream configurations using the internal logic
	configs, err := stream.GetStreamConfigs(c, options, streams)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}

func GetStream(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

	// Get stream information using internal logic
	info, err := stream.GetStreamInfo(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Build response based on advanced mode
	response := Stream{
		ModifiedAt:  info.FileInfo.ModTime(),
		Advanced:    info.Model.Advanced,
		Status:      info.Status,
		Name:        name,
		Filepath:    info.Path,
		EnvGroupID:  info.Model.EnvGroupID,
		EnvGroup:    info.Model.EnvGroup,
		SyncNodeIDs: info.Model.SyncNodeIDs,
	}

	if info.Model.Advanced {
		response.Config = info.RawContent
	} else {
		response.Config = info.NgxConfig.FmtCode()
		response.Tokenized = info.NgxConfig
	}

	c.JSON(http.StatusOK, response)
}

func SaveStream(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

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

	// Save stream configuration using internal logic
	err := stream.SaveStreamConfig(name, json.Content, json.EnvGroupID, json.SyncNodeIDs, json.Overwrite, json.PostAction)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Return the updated stream
	GetStream(c)
}

func EnableStream(c *gin.Context) {
	// Enable the stream by creating a symlink in streams-enabled directory
	err := stream.Enable(helper.UnescapeURL(c.Param("name")))
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
	err := stream.Disable(helper.UnescapeURL(c.Param("name")))
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
	err := stream.Delete(helper.UnescapeURL(c.Param("name")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func RenameStream(c *gin.Context) {
	oldName := helper.UnescapeURL(c.Param("name"))
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
