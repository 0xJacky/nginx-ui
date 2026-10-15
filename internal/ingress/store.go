package ingress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
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

// storeMu serializes the writes of the store so ownership and host checks
// see a consistent set of ingresses.
var storeMu sync.Mutex

// List returns the ingresses, all of them when owner is empty.
func List(owner string) ([]*model.Ingress, error) {
	query := model.UseDB().Order("name")
	if owner != "" {
		query = query.Where("owner = ?", owner)
	}
	items := make([]*model.Ingress, 0)
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Get returns one ingress.
func Get(name string) (*model.Ingress, error) {
	ing, err := loadIngress(name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, cosy.WrapErrorWithParams(ErrNotFound, name)
	}
	return ing, err
}

// findIngress returns the stored ingress or nil when there is none.
func findIngress(name string) (*model.Ingress, error) {
	ing, err := loadIngress(name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return ing, err
}

// desired is one ingress as a write asks for it.
type desired struct {
	name        string
	owner       string
	externalID  string
	labels      map[string]string
	spec        model.IngressSpec
	namespaceID uint64
}

func (d *desired) validate() error {
	if err := ValidateName(d.name); err != nil {
		return err
	}
	if err := ValidateOwner(d.owner); err != nil {
		return err
	}
	return Validate(&d.spec)
}

// Upsert validates and stores an ingress. The generation increases only when
// the spec or the namespace changed: those change the generated files. Owner,
// external id and labels are stored without a new generation. The ingress is
// enqueued either way so a retry re-applies.
func Upsert(ctx context.Context, name string, req UpsertRequest) (*model.Ingress, error) {
	d := desiredFromUpsert(name, req)
	if err := d.validate(); err != nil {
		return nil, err
	}

	storeMu.Lock()
	existing, err := checkUpsert(d, req.Force)
	if err == nil {
		err = store(d, existing)
	}
	storeMu.Unlock()
	if err != nil {
		return nil, err
	}

	Enqueue(name)
	return Get(name)
}

func desiredFromUpsert(name string, req UpsertRequest) *desired {
	return &desired{
		name:        name,
		owner:       req.Owner,
		externalID:  req.ExternalID,
		labels:      req.Labels,
		spec:        req.Spec,
		namespaceID: req.NamespaceID,
	}
}

// checkUpsert runs the checks that need the stored ingresses and returns the
// existing ingress, if any.
func checkUpsert(d *desired, force bool) (*model.Ingress, error) {
	existing, err := findIngress(d.name)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Owner != d.owner && !force {
		return nil, cosy.WrapErrorWithParams(ErrOwnerMismatch, d.name, existing.Owner)
	}
	all, err := List("")
	if err != nil {
		return nil, err
	}
	others := make([]*model.Ingress, 0, len(all))
	for _, ing := range all {
		if ing.Name != d.name {
			others = append(others, ing)
		}
	}
	if err := checkConflicts([]*desired{d}, others); err != nil {
		return nil, err
	}
	if err := checkSiteFile(d.name); err != nil {
		return nil, err
	}
	return existing, nil
}

// checkSiteFile refuses an ingress whose generated site name is taken by a
// site it does not manage.
func checkSiteFile(name string) error {
	sitePath, err := config.ResolveConfPathInDir("sites-available", SiteName(name))
	if err != nil {
		return err
	}
	return checkSiteOwnership(name, sitePath)
}

// checkConflicts refuses hosts served by another ingress and upstream names
// that collide with another ingress (UpstreamName("a", "b-c") equals
// UpstreamName("a-b", "c")). items are checked against others and against
// each other.
func checkConflicts(items []*desired, others []*model.Ingress) error {
	hostOwner := make(map[string]string)
	upstreamOwner := make(map[string]string)
	for _, ing := range others {
		hosts, err := normalizeHosts(&ing.Spec)
		if err == nil {
			for _, host := range hosts {
				hostOwner[host] = ing.Name
			}
		}
		for _, b := range ing.Spec.Backends {
			upstreamOwner[UpstreamName(ing.Name, b.Name)] = ing.Name
		}
	}
	for _, d := range items {
		hosts, err := normalizeHosts(&d.spec)
		if err != nil {
			return err
		}
		for _, host := range hosts {
			if owner, ok := hostOwner[host]; ok && owner != d.name {
				return cosy.WrapErrorWithParams(ErrHostConflict, host, owner)
			}
			hostOwner[host] = d.name
		}
		for _, b := range d.spec.Backends {
			upstreamName := UpstreamName(d.name, b.Name)
			if owner, ok := upstreamOwner[upstreamName]; ok && owner != d.name {
				return cosy.WrapErrorWithParams(ErrInvalidBackend, b.Name,
					fmt.Sprintf("its upstream name %s is already used by ingress %s", upstreamName, owner))
			}
			upstreamOwner[upstreamName] = d.name
		}
	}
	return nil
}

// specEqual compares two specs by their stored JSON form.
func specEqual(a, b *model.IngressSpec) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

// store creates or updates the ingress row. It never touches the status
// except to recompute the phase, which the worker owns otherwise.
func store(d *desired, existing *model.Ingress) error {
	db := model.UseDB()
	if existing == nil {
		return db.Create(&model.Ingress{
			Name:        d.name,
			Owner:       d.owner,
			ExternalID:  d.externalID,
			Labels:      d.labels,
			Spec:        d.spec,
			Generation:  1,
			SitePath:    SiteName(d.name),
			NamespaceID: d.namespaceID,
			Status:      model.IngressStatus{Phase: model.IngressPhasePending},
		}).Error
	}

	generation := existing.Generation
	if !specEqual(&existing.Spec, &d.spec) || existing.NamespaceID != d.namespaceID {
		generation++
	}
	err := db.Model(&model.Ingress{}).Where("id = ?", existing.ID).
		Select("owner", "external_id", "labels", "spec", "generation", "site_path", "namespace_id", "updated_at").
		Updates(&model.Ingress{
			Owner:       d.owner,
			ExternalID:  d.externalID,
			Labels:      d.labels,
			Spec:        d.spec,
			Generation:  generation,
			SitePath:    SiteName(d.name),
			NamespaceID: d.namespaceID,
		}).Error
	if err != nil {
		return err
	}
	if generation != existing.Generation {
		_, err = UpdateStatus(d.name, func(cur *model.Ingress) {
			if cur.Status.Phase != model.IngressPhaseApplying {
				cur.Status.Phase = ""
			}
		})
	}
	return err
}

// DryRun renders an ingress as Upsert would store it and tests it without
// storing or applying anything. Validation and ownership errors are returned
// as errors; a failed render or nginx test is reported in the result.
func DryRun(ctx context.Context, name string, req UpsertRequest) (*ApplyResult, error) {
	d := desiredFromUpsert(name, req)
	if err := d.validate(); err != nil {
		return nil, err
	}
	storeMu.Lock()
	_, err := checkUpsert(d, req.Force)
	storeMu.Unlock()
	if err != nil {
		return nil, err
	}

	ing := &model.Ingress{
		Name:        d.name,
		Owner:       d.owner,
		ExternalID:  d.externalID,
		Labels:      d.labels,
		Spec:        d.spec,
		SitePath:    SiteName(d.name),
		NamespaceID: d.namespaceID,
	}
	results, _ := Apply(ctx, []*model.Ingress{ing}, ApplyOptions{DryRun: true})
	if len(results) == 0 {
		return nil, cosy.WrapErrorWithParams(ErrApplyFailed, "dry run returned no result")
	}
	return &results[0], nil
}

// Delete removes an ingress and its files. The certificate row is kept unless
// purgeCert is set. It returns once the worker removed the files and the
// row, or ctx ends.
func Delete(ctx context.Context, name, owner string, force, purgeCert bool) error {
	ing, err := Get(name)
	if err != nil {
		return err
	}
	if ing.Owner != owner && !force {
		return cosy.WrapErrorWithParams(ErrOwnerMismatch, name, ing.Owner)
	}
	done := enqueueDelete(name, purgeCert)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SyncOwner makes the owner's ingresses match items: upserts every item and,
// when prune is set, deletes the owner's ingresses missing from items. All
// changes are applied with one reload. Items owned by another owner are
// refused. The result lists the owner's ingresses after the sync, without the
// ones queued for deletion.
func SyncOwner(ctx context.Context, owner string, req SyncRequest, prune bool) ([]*model.Ingress, error) {
	if err := ValidateOwner(owner); err != nil {
		return nil, err
	}
	items := make([]*desired, 0, len(req.Items))
	listed := make(map[string]struct{}, len(req.Items))
	for _, item := range req.Items {
		d := &desired{
			name:        item.Name,
			owner:       owner,
			externalID:  item.ExternalID,
			labels:      item.Labels,
			spec:        item.Spec,
			namespaceID: item.NamespaceID,
		}
		if err := d.validate(); err != nil {
			return nil, err
		}
		if _, ok := listed[d.name]; ok {
			return nil, cosy.WrapErrorWithParams(ErrInvalidName, d.name)
		}
		listed[d.name] = struct{}{}
		items = append(items, d)
	}

	storeMu.Lock()
	pruned, err := syncLocked(owner, items, listed, prune)
	storeMu.Unlock()
	if err != nil {
		return nil, err
	}

	for _, name := range pruned {
		enqueueDelete(name, false)
	}
	names := make([]string, 0, len(items))
	for _, d := range items {
		names = append(names, d.name)
	}
	Enqueue(names...)

	all, err := List(owner)
	if err != nil {
		return nil, err
	}
	prunedSet := make(map[string]struct{}, len(pruned))
	for _, name := range pruned {
		prunedSet[name] = struct{}{}
	}
	result := make([]*model.Ingress, 0, len(all))
	for _, ing := range all {
		if _, ok := prunedSet[ing.Name]; !ok {
			result = append(result, ing)
		}
	}
	return result, nil
}

// syncLocked checks and stores the items of a sync and returns the names of
// the owner's ingresses to prune.
func syncLocked(owner string, items []*desired, listed map[string]struct{}, prune bool) ([]string, error) {
	all, err := List("")
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*model.Ingress, len(all))
	var pruned []string
	others := make([]*model.Ingress, 0, len(all))
	for _, ing := range all {
		byName[ing.Name] = ing
		_, isListed := listed[ing.Name]
		switch {
		case isListed:
		case prune && ing.Owner == owner:
			pruned = append(pruned, ing.Name)
		default:
			others = append(others, ing)
		}
	}

	for _, d := range items {
		if existing := byName[d.name]; existing != nil && existing.Owner != owner {
			return nil, cosy.WrapErrorWithParams(ErrOwnerMismatch, d.name, existing.Owner)
		}
		if err := checkSiteFile(d.name); err != nil {
			return nil, err
		}
	}
	if err := checkConflicts(items, others); err != nil {
		return nil, err
	}

	for _, d := range items {
		if err := store(d, byName[d.name]); err != nil {
			return nil, err
		}
	}
	return pruned, nil
}

// PatchServer marks one server of a backend down or up. It bumps the
// generation like a spec change when the flag changes.
func PatchServer(ctx context.Context, name, backend string, patch ServerPatch) (*model.Ingress, error) {
	if err := ValidateOwner(patch.Owner); err != nil {
		return nil, err
	}

	storeMu.Lock()
	err := patchServerLocked(name, backend, patch)
	storeMu.Unlock()
	if err != nil {
		return nil, err
	}

	Enqueue(name)
	return Get(name)
}

func patchServerLocked(name, backend string, patch ServerPatch) error {
	ing, err := Get(name)
	if err != nil {
		return err
	}
	if ing.Owner != patch.Owner {
		return cosy.WrapErrorWithParams(ErrOwnerMismatch, name, ing.Owner)
	}

	spec := ing.Spec
	spec.Backends = append([]model.IngressBackend(nil), ing.Spec.Backends...)
	for i := range spec.Backends {
		if spec.Backends[i].Name != backend {
			continue
		}
		servers := append([]model.IngressServer(nil), spec.Backends[i].Servers...)
		for j := range servers {
			if !sameAddress(servers[j].Address, patch.Address) {
				continue
			}
			servers[j].Down = patch.Down
			spec.Backends[i].Servers = servers
			return store(&desired{
				name:        ing.Name,
				owner:       ing.Owner,
				externalID:  ing.ExternalID,
				labels:      ing.Labels,
				spec:        spec,
				namespaceID: ing.NamespaceID,
			}, ing)
		}
	}
	return cosy.WrapErrorWithParams(ErrServerNotFound, patch.Address, backend)
}

// sameAddress compares two server addresses by their canonical form, so
// "[::1]:80" matches "[0:0::1]:80".
func sameAddress(a, b string) bool {
	if a == b {
		return true
	}
	pa, errA := parseServerAddress(a)
	pb, errB := parseServerAddress(b)
	return errA == nil && errB == nil && pa == pb
}
