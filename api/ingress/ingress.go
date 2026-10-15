package ingress

import (
	"github.com/0xJacky/Nginx-UI/internal/ingress"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// ListIngresses handles GET /ingresses?owner=.
func ListIngresses(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }

// GetIngress handles GET /ingresses/:name.
func GetIngress(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }

// SyncIngresses handles PUT /ingresses?owner=&prune=.
func SyncIngresses(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }

// PutIngress handles PUT /ingresses/:name?wait=&dry_run=.
func PutIngress(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }

// DeleteIngress handles DELETE /ingresses/:name?owner=&force=&purge_cert=.
func DeleteIngress(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }

// PatchIngressServer handles PATCH /ingresses/:name/backends/:backend/servers.
func PatchIngressServer(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }

// ReconcileIngress handles POST /ingresses/:name/reconcile.
func ReconcileIngress(c *gin.Context) { cosy.ErrHandler(c, ingress.ErrNotImplemented) }
