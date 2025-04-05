package sites

import (
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm/clause"
)

func GetSite(c *gin.Context) {
	name := c.Param("name")

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
		cosy.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	certModel, err := model.FirstCert(name)
	if err != nil {
		logger.Warn(err)
	}

	if siteModel.Advanced {
		origContent, err := os.ReadFile(path)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		c.JSON(http.StatusOK, Site{
			ModifiedAt:      file.ModTime(),
			Site:            siteModel,
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
		cosy.ErrHandler(c, err)
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
		Site:            siteModel,
		ModifiedAt:      file.ModTime(),
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

	var json struct {
		Content     string   `json:"content" binding:"required"`
		EnvGroupID  uint64   `json:"env_group_id"`
		SyncNodeIDs []uint64 `json:"sync_node_ids"`
		Overwrite   bool     `json:"overwrite"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := site.Save(name, json.Content, json.Overwrite, json.EnvGroupID, json.SyncNodeIDs)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	GetSite(c)
}

func RenameSite(c *gin.Context) {
	oldName := c.Param("name")
	var json struct {
		NewName string `json:"new_name"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := site.Rename(oldName, json.NewName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func EnableSite(c *gin.Context) {
	err := site.Enable(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DisableSite(c *gin.Context) {
	err := site.Disable(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteSite(c *gin.Context) {
	err := site.Delete(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func BatchUpdateSites(c *gin.Context) {
	cosy.Core[model.Site](c).SetValidRules(gin.H{
		"env_group_id": "required",
	}).SetItemKey("path").
		BeforeExecuteHook(func(ctx *cosy.Ctx[model.Site]) {
			effectedPath := make([]string, len(ctx.BatchEffectedIDs))
			var sites []*model.Site
			for i, name := range ctx.BatchEffectedIDs {
				path := nginx.GetConfPath("sites-available", name)
				effectedPath[i] = path
				sites = append(sites, &model.Site{
					Path: path,
				})
			}
			s := query.Site
			err := s.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(sites...)
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			ctx.BatchEffectedIDs = effectedPath
		}).BatchModify()
}
