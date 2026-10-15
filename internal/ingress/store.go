package ingress

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
)

// UpsertRequest is the body of PUT /api/ingresses/:name.
type UpsertRequest struct {
	Owner       string            `json:"owner" binding:"required"`
	ExternalID  string            `json:"external_id"`
	Labels      map[string]string `json:"labels"`
	Spec        model.IngressSpec `json:"spec" binding:"required"`
	NamespaceID uint64            `json:"namespace_id"`
	// Force takes over an ingress owned by someone else.
	Force bool `json:"force"`
}

// SyncItem is one ingress of PUT /api/ingresses.
type SyncItem struct {
	Name        string            `json:"name" binding:"required"`
	ExternalID  string            `json:"external_id"`
	Labels      map[string]string `json:"labels"`
	Spec        model.IngressSpec `json:"spec" binding:"required"`
	NamespaceID uint64            `json:"namespace_id"`
}

// SyncRequest is the body of PUT /api/ingresses?owner=&prune=.
type SyncRequest struct {
	Items []SyncItem `json:"items"`
}

// ServerPatch is the body of PATCH /api/ingresses/:name/backends/:backend/servers.
type ServerPatch struct {
	Owner   string `json:"owner" binding:"required"`
	Address string `json:"address" binding:"required"`
	Down    bool   `json:"down"`
}

// List returns the ingresses, all of them when owner is empty.
func List(owner string) ([]*model.Ingress, error) {
	return nil, ErrNotImplemented
}

// Get returns one ingress.
func Get(name string) (*model.Ingress, error) {
	return nil, ErrNotImplemented
}

// Upsert validates and stores an ingress. The generation increases only when
// the spec changed; the ingress is enqueued either way so a retry re-applies.
func Upsert(ctx context.Context, name string, req UpsertRequest) (*model.Ingress, error) {
	return nil, ErrNotImplemented
}

// DryRun renders an ingress as Upsert would store it and tests it without
// storing or applying anything.
func DryRun(ctx context.Context, name string, req UpsertRequest) (*ApplyResult, error) {
	return nil, ErrNotImplemented
}

// Delete removes an ingress and its files. The certificate row is kept unless
// purgeCert is set.
func Delete(ctx context.Context, name, owner string, force, purgeCert bool) error {
	return ErrNotImplemented
}

// SyncOwner makes the owner's ingresses match items: upserts every item and,
// when prune is set, deletes the owner's ingresses missing from items. All
// changes are applied with one reload.
func SyncOwner(ctx context.Context, owner string, req SyncRequest, prune bool) ([]*model.Ingress, error) {
	return nil, ErrNotImplemented
}

// PatchServer marks one server of a backend down or up. It bumps the
// generation like a spec change.
func PatchServer(ctx context.Context, name, backend string, patch ServerPatch) (*model.Ingress, error) {
	return nil, ErrNotImplemented
}
