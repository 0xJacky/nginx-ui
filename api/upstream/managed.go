package upstream

import (
	"net/http"
	"path/filepath"
	"sort"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// ExternalUpstream is an upstream block defined outside the managed files, for
// example inside a site configuration. It is listed read-only.
type ExternalUpstream struct {
	Name       string                 `json:"name"`
	Servers    []upstream.ProxyTarget `json:"servers"`
	ConfigPath string                 `json:"config_path"`
}

// ListManagedUpstreams returns the upstreams managed by Nginx UI plus the
// upstream blocks found elsewhere in the configuration.
func ListManagedUpstreams(c *gin.Context) {
	items, err := managed.List()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	managedPaths := make(map[string]bool, len(items))
	for _, item := range items {
		managedPaths[filepath.Clean(item.Path)] = true
	}

	external := make([]ExternalUpstream, 0)
	for name, def := range upstream.GetUpstreamService().GetAllUpstreamDefinitions() {
		if managedPaths[filepath.Clean(def.ConfigPath)] {
			continue
		}
		external = append(external, ExternalUpstream{
			Name:       name,
			Servers:    def.Servers,
			ConfigPath: def.ConfigPath,
		})
	}
	sort.Slice(external, func(i, j int) bool { return external[i].Name < external[j].Name })

	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"external": external,
		"dir":      managed.Dir(),
	})
}

// GetManagedUpstream returns one managed upstream with its references.
func GetManagedUpstream(c *gin.Context) {
	detail, err := managed.Get(c.Param("name"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// createUpstreamRequest lets a create call leave out `zone`: the outer field
// shadows Upstream.Zone when decoding, so its absence can be told apart from
// an explicit false.
type createUpstreamRequest struct {
	managed.Upstream
	Zone *bool `json:"zone"`
}

// CreateManagedUpstream adds a new upstream group. New groups get a shared
// memory zone unless the request turns it off explicitly.
func CreateManagedUpstream(c *gin.Context) {
	var req createUpstreamRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	req.Upstream.Zone = req.Zone == nil || *req.Zone

	detail, err := managed.Save(&req.Upstream, true, api.CurrentUser(c).Name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// UpdateManagedUpstream replaces the servers and settings of an upstream. The
// name in the path wins over the body, because renaming would orphan every
// site that references the group.
func UpdateManagedUpstream(c *gin.Context) {
	var req managed.Upstream
	if !cosy.BindAndValid(c, &req) {
		return
	}
	req.Name = c.Param("name")

	detail, err := managed.Save(&req, false, api.CurrentUser(c).Name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// PreviewManagedUpstream validates an upstream and returns the configuration
// it would be saved as, without touching the disk.
func PreviewManagedUpstream(c *gin.Context) {
	var req managed.Upstream
	if !cosy.BindAndValid(c, &req) {
		return
	}

	content, err := managed.Prepare(&req)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content":   content,
		"file_name": managed.FileName(req.Name),
	})
}

// DeleteManagedUpstream removes an upstream that no site references anymore.
func DeleteManagedUpstream(c *gin.Context) {
	if err := managed.Delete(c.Param("name")); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
