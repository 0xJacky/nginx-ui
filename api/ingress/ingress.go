package ingress

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/ingress"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// maxWait caps the ?wait= duration.
const maxWait = 60 * time.Second

// respondError answers with the HTTP status given by the first three digits
// of a cosy error code (40901 -> 409), so controllers can rely on the status
// alone. Other errors are 500.
func respondError(c *gin.Context, err error) {
	logger.NewSessionLogger(c).Error(err)

	var cErr *cosy.Error
	switch {
	case errors.As(err, &cErr):
		c.JSON(statusOf(cErr.Code), cErr)
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, &cosy.Error{Code: http.StatusNotFound, Message: err.Error()})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		c.JSON(http.StatusRequestTimeout, &cosy.Error{Code: http.StatusRequestTimeout, Message: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, &cosy.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
}

// statusOf maps a five digit error code to its HTTP status.
func statusOf(code int32) int {
	status := int(code)
	for status >= 1000 {
		status /= 10
	}
	if status < 400 || status > 599 {
		return http.StatusInternalServerError
	}
	return status
}

func badRequest(c *gin.Context, err error) {
	respondError(c, cosy.WrapErrorWithParams(ingress.ErrInvalidRequest, err.Error()))
}

func queryFlag(c *gin.Context, key string) bool {
	switch c.Query(key) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// parseWait reads ?wait= as a Go duration capped at maxWait. Zero means do
// not wait.
func parseWait(c *gin.Context) (time.Duration, error) {
	raw := c.Query("wait")
	if raw == "" {
		return 0, nil
	}
	wait, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errors.New("wait must be a duration such as 10s")
	}
	if wait < 0 {
		return 0, errors.New("wait must not be negative")
	}
	return min(wait, maxWait), nil
}

// waitFor blocks until every ingress reached its generation through an apply
// enqueued after seq (ingress.ApplySeq before the request's write) and every
// name in deleted is gone, or the wait expires. It returns the current state
// of the ingresses.
func waitFor(c *gin.Context, wait time.Duration, seq uint64, items []*model.Ingress, deleted []string) ([]*model.Ingress, error) {
	if wait <= 0 {
		return items, nil
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), wait)
	defer cancel()
	result := make([]*model.Ingress, 0, len(items))
	for _, item := range items {
		ing, err := ingress.WaitForApply(ctx, item.Name, item.Generation, seq)
		if err != nil {
			return nil, err
		}
		result = append(result, ing)
	}
	for _, name := range deleted {
		if err := ingress.WaitForDeletion(ctx, name); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// ListIngresses handles GET /ingresses?owner=.
func ListIngresses(c *gin.Context) {
	items, err := ingress.List(c.Query("owner"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetIngress handles GET /ingresses/:name.
func GetIngress(c *gin.Context) {
	ing, err := ingress.Get(c.Param("name"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, ing)
}

// SyncIngresses handles PUT /ingresses?owner=&prune=.
func SyncIngresses(c *gin.Context) {
	wait, err := parseWait(c)
	if err != nil {
		badRequest(c, err)
		return
	}
	var req ingress.SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	seq := ingress.ApplySeq()
	items, pruned, err := ingress.SyncOwnerPruned(c.Request.Context(), c.Query("owner"), req, queryFlag(c, "prune"))
	if err != nil {
		respondError(c, err)
		return
	}
	items, err = waitFor(c, wait, seq, items, pruned)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// PutIngress handles PUT /ingresses/:name?wait=&dry_run=.
func PutIngress(c *gin.Context) {
	wait, err := parseWait(c)
	if err != nil {
		badRequest(c, err)
		return
	}
	var req ingress.UpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	name := c.Param("name")

	if queryFlag(c, "dry_run") {
		result, err := ingress.DryRun(c.Request.Context(), name, req)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	seq := ingress.ApplySeq()
	ing, err := ingress.Upsert(c.Request.Context(), name, req)
	if err != nil {
		respondError(c, err)
		return
	}
	items, err := waitFor(c, wait, seq, []*model.Ingress{ing}, nil)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items[0])
}

// DeleteIngress handles DELETE /ingresses/:name?owner=&force=&purge_cert=.
func DeleteIngress(c *gin.Context) {
	owner := c.Query("owner")
	force := queryFlag(c, "force")
	if !force {
		if err := ingress.ValidateOwner(owner); err != nil {
			respondError(c, err)
			return
		}
	}
	err := ingress.Delete(c.Request.Context(), c.Param("name"), owner, force, queryFlag(c, "purge_cert"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// PatchIngressServer handles PATCH /ingresses/:name/backends/:backend/servers.
func PatchIngressServer(c *gin.Context) {
	wait, err := parseWait(c)
	if err != nil {
		badRequest(c, err)
		return
	}
	var patch ingress.ServerPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, err)
		return
	}
	seq := ingress.ApplySeq()
	ing, err := ingress.PatchServer(c.Request.Context(), c.Param("name"), c.Param("backend"), patch)
	if err != nil {
		respondError(c, err)
		return
	}
	items, err := waitFor(c, wait, seq, []*model.Ingress{ing}, nil)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items[0])
}

// ReconcileIngress handles POST /ingresses/:name/reconcile?wait=: it queues
// the ingress for apply, which rewrites its files when they drifted.
func ReconcileIngress(c *gin.Context) {
	wait, err := parseWait(c)
	if err != nil {
		badRequest(c, err)
		return
	}
	ing, err := ingress.Get(c.Param("name"))
	if err != nil {
		respondError(c, err)
		return
	}
	seq := ingress.ApplySeq()
	ingress.Enqueue(ing.Name)
	items, err := waitFor(c, wait, seq, []*model.Ingress{ing}, nil)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items[0])
}
