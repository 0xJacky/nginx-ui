package access_list

import (
	"net/http"

	internalaccess "github.com/0xJacky/Nginx-UI/internal/access_list"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// controlPayload carries the editor content. Advanced mode sends the file
// text, basic mode sends the structured configuration it edits.
type controlPayload struct {
	Content *string                 `json:"content"`
	Config  *nginx.NgxConfig        `json:"config"`
	Changes []internalaccess.Change `json:"changes"`
}

type controlResponse struct {
	Content *string                      `json:"content,omitempty"`
	Config  *nginx.NgxConfig             `json:"config,omitempty"`
	Servers []internalaccess.ServerState `json:"servers"`
}

func stateOf(payload *controlPayload) (*controlResponse, error) {
	switch {
	case payload.Config != nil:
		servers, err := internalaccess.StateOf(payload.Config)
		if err != nil {
			return nil, err
		}
		return &controlResponse{Config: payload.Config, Servers: servers}, nil
	case payload.Content != nil:
		servers, err := internalaccess.State(*payload.Content)
		if err != nil {
			return nil, err
		}
		return &controlResponse{Content: payload.Content, Servers: servers}, nil
	default:
		return &controlResponse{Servers: []internalaccess.ServerState{}}, nil
	}
}

// GetAccessControlState reports which access lists the servers and locations
// of the editor content use.
func GetAccessControlState(c *gin.Context) {
	var json controlPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	resp, err := stateOf(&json)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	resp.Config, resp.Content = nil, nil
	c.JSON(http.StatusOK, resp)
}

// ApplyAccessControl rewrites the access directives of the editor content and
// returns the new content together with its state.
func ApplyAccessControl(c *gin.Context) {
	var json controlPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}

	if err := internalaccess.SlugsExist(changeSlugs(json.Changes)); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	switch {
	case json.Config != nil:
		if err := internalaccess.ApplyTo(json.Config, json.Changes); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	case json.Content != nil:
		content, err := internalaccess.Apply(*json.Content, json.Changes)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		json.Content = &content
	}

	resp, err := stateOf(&json)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func changeSlugs(changes []internalaccess.Change) []string {
	var slugs []string
	for _, change := range changes {
		if change.Mode == internalaccess.ModeList {
			slugs = append(slugs, change.Slug)
		}
	}
	return slugs
}

// batchResult reports the outcome for one site or stream.
type batchResult struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BatchApplyAccessControl sets the server level access of every server block
// of the selected sites or streams, saving each file through the regular save
// path so it is tested, reloaded and synchronized.
func BatchApplyAccessControl(c *gin.Context) {
	var json struct {
		Kind  string   `json:"kind" binding:"required,oneof=site stream"`
		Names []string `json:"names" binding:"required,min=1,max=500"`
		Mode  string   `json:"mode" binding:"required,oneof=public list"`
		Slug  string   `json:"slug"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}
	if json.Mode == internalaccess.ModeList {
		if err := internalaccess.SlugsExist([]string{json.Slug}); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	results := make([]batchResult, 0, len(json.Names))
	for _, name := range json.Names {
		err := batchApplyOne(json.Kind, name, json.Mode, json.Slug)
		result := batchResult{Name: name, Success: err == nil}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func batchApplyOne(kind, name, mode, slug string) error {
	resolve := site.ResolveAvailablePath
	if kind == internalaccess.RefKindStream {
		resolve = stream.ResolveAvailablePath
	}
	path, err := resolve(name)
	if err != nil {
		return err
	}
	raw, err := nginx.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(raw)

	servers, err := internalaccess.State(content)
	if err != nil {
		return err
	}
	changes := make([]internalaccess.Change, 0, len(servers))
	for _, server := range servers {
		if server.Mode == mode && server.Slug == slug {
			continue
		}
		changes = append(changes, internalaccess.Change{Server: server.Index, Mode: mode, Slug: slug})
	}
	if len(changes) == 0 {
		return nil
	}
	updated, err := internalaccess.Apply(content, changes)
	if err != nil {
		return err
	}

	if kind == internalaccess.RefKindStream {
		s := query.Stream
		var syncNodeIDs []uint64
		if record, err := s.Where(s.Path.Eq(path)).First(); err == nil {
			syncNodeIDs = record.SyncNodeIDs
		}
		return stream.Save(name, updated, true, syncNodeIDs, model.PostSyncActionReloadNginx)
	}

	s := query.Site
	var namespaceID uint64
	var syncNodeIDs []uint64
	if record, err := s.Where(s.Path.Eq(path)).First(); err == nil {
		namespaceID = record.NamespaceID
		syncNodeIDs = record.SyncNodeIDs
	}
	return site.Save(name, updated, true, namespaceID, syncNodeIDs, model.PostSyncActionReloadNginx)
}
