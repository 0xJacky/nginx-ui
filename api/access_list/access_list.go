package access_list

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	internalaccess "github.com/0xJacky/Nginx-UI/internal/access_list"
	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// accessListItem is a list as the list page shows it.
type accessListItem struct {
	*model.AccessList
	Warnings []internalaccess.Warning `json:"warnings"`
	// UsedBy counts the lists, sites and streams that use the list.
	UsedBy int `json:"used_by"`
}

type accessListPayload struct {
	Name     string             `json:"name" binding:"required,max=100"`
	Slug     string             `json:"slug"`
	Fallback string             `json:"fallback"`
	Rules    []model.AccessRule `json:"rules" binding:"max=1000"`
}

func (p accessListPayload) toModel(id uint64) *model.AccessList {
	rules := make([]model.AccessRule, 0, len(p.Rules))
	for _, rule := range p.Rules {
		rule.Value = strings.TrimSpace(rule.Value)
		rule.Note = strings.TrimSpace(rule.Note)
		if rule.Type == model.AccessRuleRef {
			rule.Value = ""
		} else {
			rule.RefID = 0
		}
		rules = append(rules, rule)
	}
	return &model.AccessList{
		Model:    model.Model{ID: id},
		Name:     p.Name,
		Slug:     strings.TrimSpace(p.Slug),
		Fallback: p.Fallback,
		Rules:    rules,
	}
}

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		cosy.ErrHandler(c, internalaccess.ErrAccessListNotFound)
		return 0, false
	}
	return id, true
}

// GetAccessLists returns every list. Lists are few, so the whole set is
// returned as one page.
func GetAccessLists(c *gin.Context) {
	lists, err := internalaccess.All()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	usage := map[string]int{}
	for _, ref := range internalaccess.ScanReferences() {
		usage[ref.Slug]++
	}

	items := make([]accessListItem, 0, len(lists))
	for _, l := range lists {
		items = append(items, accessListItem{
			AccessList: l,
			Warnings:   internalaccess.Warnings(l),
			UsedBy:     usage[l.Slug] + len(internalaccess.DirectDependents(l.ID, lists)),
		})
	}

	c.JSON(http.StatusOK, model.DataList{
		Data: items,
		Pagination: model.Pagination{
			Total:       int64(len(items)),
			PerPage:     len(items),
			CurrentPage: 1,
			TotalPages:  1,
		},
	})
}

// GetAccessList returns one list.
func GetAccessList(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	list, err := internalaccess.Get(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, accessListItem{AccessList: list, Warnings: internalaccess.Warnings(list)})
}

// GetAccessListUsage returns the lists, sites and streams that use a list.
func GetAccessListUsage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	usage, err := internalaccess.UsageOf(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, usage)
}

// PreviewAccessList renders a list without saving it.
func PreviewAccessList(c *gin.Context) {
	var json struct {
		accessListPayload
		ID uint64 `json:"id"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}
	content, warnings, err := internalaccess.Preview(json.toModel(json.ID))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content":  content,
		"warnings": warnings,
	})
}

// CreateAccessList saves a new list.
func CreateAccessList(c *gin.Context) {
	var json accessListPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	saveAccessList(c, json.toModel(0))
}

// ModifyAccessList updates a list. The slug is fixed at creation and ignored.
func ModifyAccessList(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var json accessListPayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	saveAccessList(c, json.toModel(id))
}

func saveAccessList(c *gin.Context, list *model.AccessList) {
	result, err := internalaccess.Save(list)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	go syncAccessLists(result.Slugs, result.References)

	c.JSON(http.StatusOK, accessListItem{AccessList: result.List, Warnings: internalaccess.Warnings(result.List)})
}

// syncAccessLists replicates changed list files to the nodes of the sites and
// streams that include them.
func syncAccessLists(slugs []string, refs []internalaccess.Reference) {
	seen := map[uint64]bool{}
	var nodeIDs []uint64
	for _, ref := range refs {
		var nodes []*model.Node
		switch ref.Kind {
		case internalaccess.RefKindSite:
			nodes = site.GetSyncNodes(ref.Name)
		case internalaccess.RefKindStream:
			nodes = stream.GetSyncNodes(ref.Name)
		}
		for _, node := range nodes {
			if !seen[node.ID] {
				seen[node.ID] = true
				nodeIDs = append(nodeIDs, node.ID)
			}
		}
	}
	if len(nodeIDs) == 0 {
		return
	}
	summary, err := clustersync.PushAccessLists(context.Background(), slugs, nodeIDs)
	if err != nil {
		logger.Errorf("Syncing access lists failed: %v", err)
		return
	}
	if summary.Failed > 0 {
		logger.Errorf("Syncing access lists failed on %d of %d node pushes", summary.Failed, summary.Total)
	}
}

// DeleteAccessList removes a list nothing uses.
func DeleteAccessList(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := internalaccess.Delete(id); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
