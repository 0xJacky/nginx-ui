package snippet

import (
	"context"
	"net/http"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/snippet"
	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type snippetPayload struct {
	File        string            `json:"file"`
	NameI18n    map[string]string `json:"name_i18n"`
	Description map[string]string `json:"description"`
	// Author and Variables are kept when left out.
	Author    *string                      `json:"author"`
	Variables map[string]template.Variable `json:"variables"`
	Content   string                       `json:"content"`
}

// GetSnippets lists the snippets.
func GetSnippets(c *gin.Context) {
	snippets, err := snippet.List()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, model.DataList{
		Data: snippets,
		Pagination: model.Pagination{
			Total:       int64(len(snippets)),
			PerPage:     len(snippets),
			CurrentPage: 1,
			TotalPages:  1,
		},
	})
}

// GetSnippet returns one snippet with its body.
func GetSnippet(c *gin.Context) {
	s, err := snippet.Get(c.Param("file"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// CreateSnippet writes a new snippet.
func CreateSnippet(c *gin.Context) {
	var json snippetPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	save(c, json.File, json, true)
}

// ModifySnippet replaces the header and body of a snippet, and renames it
// when the payload names another file and nothing includes the snippet.
func ModifySnippet(c *gin.Context) {
	var json snippetPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	file := c.Param("file")
	if json.File == "" || json.File == file {
		save(c, file, json, false)
		return
	}
	// A new file name renames the snippet: the new file is written first, so
	// a configuration nginx rejects leaves the old one in place.
	if err := snippet.CheckRename(file, json.File); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	s, err := saveSnippet(c, json.File, json, true)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if err = removeSnippet(c.Request.Context(), file); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func save(c *gin.Context, file string, json snippetPayload, create bool) {
	s, err := saveSnippet(c, file, json, create)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func saveSnippet(c *gin.Context, file string, json snippetPayload, create bool) (snippet.Snippet, error) {
	return snippet.Save(c.Request.Context(), snippet.SaveParams{
		File:        file,
		Names:       json.NameI18n,
		Description: json.Description,
		Author:      json.Author,
		Variables:   json.Variables,
		Content:     json.Content,
		Create:      create,
	}, api.CurrentUser(c).Name)
}

// GetBuiltinTemplates lists the block templates built into Nginx UI, which
// a snippet can start from.
func GetBuiltinTemplates(c *gin.Context) {
	list, err := template.GetTemplateList("block")
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// GetBuiltinTemplate returns a built-in block template as written.
func GetBuiltinTemplate(c *gin.Context) {
	info, body, err := template.BuiltinBlockSource(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, snippet.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"name":        info.Name,
		"description": info.Description,
		"author":      info.Author,
		"filename":    info.Filename,
		"variables":   info.Variables,
		"content":     body,
	})
}

type previewPayload struct {
	Content   string                       `json:"content"`
	Variables map[string]template.Variable `json:"variables"`
}

// PreviewSnippet renders the content of a snippet with variable values,
// before it is saved.
func PreviewSnippet(c *gin.Context) {
	var json previewPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	result, err := snippet.Preview(json.Content, json.Variables)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// DeleteSnippet removes a snippet nothing includes, here and on the nodes the
// snippets directory is deployed to.
func DeleteSnippet(c *gin.Context) {
	if err := removeSnippet(c.Request.Context(), c.Param("file")); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// removeSnippet removes a snippet here and on the nodes it is deployed to.
func removeSnippet(ctx context.Context, file string) error {
	removed, err := snippet.Delete(file, snippet.NginxRemover{})
	if err != nil {
		return err
	}
	// Targets set on the file itself count as well as those of the directory.
	nodeIDs, _ := config.InheritedSyncTargets(removed)
	q := query.Config
	if record, findErr := q.Where(q.Filepath.Eq(removed)).First(); findErr == nil {
		nodeIDs, _ = config.EffectiveSyncTargets(record)
	}
	if err = config.CleanupDatabaseRecords(removed, false); err != nil {
		return err
	}
	if len(nodeIDs) > 0 {
		return config.SyncDeleteOnRemoteServer(ctx, removed, nodeIDs)
	}
	return nil
}

type syncPayload struct {
	SyncNodeIds   []uint64 `json:"sync_node_ids"`
	SyncOverwrite bool     `json:"sync_overwrite"`
}

// GetSnippetSync returns the nodes the snippets directory is deployed to.
func GetSnippetSync(c *gin.Context) {
	q := query.Config
	record, err := q.Where(q.Filepath.Eq(snippet.Dir()), q.IsDir.Is(true)).First()
	if err != nil {
		c.JSON(http.StatusOK, syncPayload{SyncNodeIds: []uint64{}})
		return
	}
	nodeIDs := record.SyncNodeIds
	if nodeIDs == nil {
		nodeIDs = []uint64{}
	}
	c.JSON(http.StatusOK, syncPayload{SyncNodeIds: nodeIDs, SyncOverwrite: record.SyncOverwrite})
}

// SaveSnippetSync deploys the snippets directory to the given nodes now and
// remembers them, so every later change of a snippet reaches them too. An
// empty list stops the deployment.
func SaveSnippetSync(c *gin.Context) {
	var json syncPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	dir := snippet.Dir()
	var summary *clustersync.Summary
	if len(json.SyncNodeIds) > 0 {
		if err := nginx.MkdirAll(dir, 0o755); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		var err error
		if summary, err = clustersync.SyncDirectory(c, dir, json.SyncNodeIds, json.SyncOverwrite); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}
	if err := config.SaveDirectorySyncTargets(dir, json.SyncNodeIds, json.SyncOverwrite); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if summary == nil {
		summary = &clustersync.Summary{Results: []clustersync.Result{}}
	}
	c.JSON(http.StatusOK, summary)
}
