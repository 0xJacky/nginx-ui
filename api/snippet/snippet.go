package snippet

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/snippet"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type snippetPayload struct {
	File        string            `json:"file"`
	Name        string            `json:"name"`
	Description map[string]string `json:"description"`
	Content     string            `json:"content"`
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

// ModifySnippet replaces the name, description and body of a snippet.
func ModifySnippet(c *gin.Context) {
	var json snippetPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	save(c, c.Param("file"), json, false)
}

func save(c *gin.Context, file string, json snippetPayload, create bool) {
	s, err := snippet.Save(c.Request.Context(), snippet.SaveParams{
		File:        file,
		Name:        json.Name,
		Description: json.Description,
		Content:     json.Content,
		Create:      create,
	}, api.CurrentUser(c).Name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// DeleteSnippet removes a snippet nothing includes, here and on the nodes the
// snippets directory is deployed to.
func DeleteSnippet(c *gin.Context) {
	removed, err := snippet.Delete(c.Param("file"), snippet.NginxRemover{})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// Targets set on the file itself count as well as those of the directory.
	nodeIDs, _ := config.InheritedSyncTargets(removed)
	q := query.Config
	if record, findErr := q.Where(q.Filepath.Eq(removed)).First(); findErr == nil {
		nodeIDs, _ = config.EffectiveSyncTargets(record)
	}
	if err = config.CleanupDatabaseRecords(removed, false); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if len(nodeIDs) > 0 {
		if err = config.SyncDeleteOnRemoteServer(c.Request.Context(), removed, nodeIDs); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
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
