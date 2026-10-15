package ingress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// Reasons of the ConfigApplied condition.
const (
	applyReasonApplied      = "Applied"
	applyReasonTestFailed   = "TestFailed"
	applyReasonReloadFailed = "ReloadFailed"
	applyReasonRenderFailed = "RenderFailed"
)

// itemResult is the outcome of one item of a batch with the reason the
// worker records in the ConfigApplied condition.
type itemResult struct {
	ApplyResult
	// Reason is one of the Reason* constants; empty for a successful remove.
	Reason string
	// failed is true when the item was not applied.
	failed bool
}

// batchResult is the outcome of runBatch.
type batchResult struct {
	applies map[string]*itemResult
	removes map[string]*itemResult
	// txFailed is set when the combined test or reload failed, so every item
	// of the transaction was rolled back.
	txFailed bool
	// txReason is applyReasonTestFailed or applyReasonReloadFailed when txFailed.
	txReason string
	// changed lists the applied ingresses whose files changed.
	changed []*preparedItem
	// removed lists the removed ingresses whose files were deleted.
	removed []*preparedItem
}

// preparedItem is an ingress rendered and resolved to its file paths.
type preparedItem struct {
	ing         *model.Ingress
	rendered    *Rendered
	sitePath    string
	enabledPath string
	// maintenancePath is the maintenance variant in sites-enabled.
	maintenancePath string
	// upstreams maps the upstream names to their file paths.
	upstreams map[string]string
	namespace *model.Namespace
	result    *itemResult
	changed   bool
}

// Apply renders the given ingresses and writes their files under one apply
// lock and file transaction, enabling each site, then tests and reloads nginx
// once. When the test fails every file is rolled back. It upserts the
// generated site rows but does not touch the ingress rows; the caller records
// the results in each ingress status. The error is ErrApplyFailed when any
// item failed; the results always describe every item.
func Apply(ctx context.Context, items []*model.Ingress, opts ApplyOptions) ([]ApplyResult, error) {
	certStates := resolveCerts(ctx, items, opts.Certs)
	res := runBatch(ctx, items, nil, certStates, opts.DryRun)

	results := make([]ApplyResult, 0, len(items))
	var failures []string
	for _, ing := range items {
		r := res.applies[ing.Name]
		results = append(results, r.ApplyResult)
		if r.failed {
			failures = append(failures, ing.Name+": "+r.Error)
		}
	}
	if len(failures) > 0 {
		return results, cosy.WrapErrorWithParams(ErrApplyFailed, strings.Join(failures, "; "))
	}
	return results, nil
}

// Remove deletes the generated files of the given ingresses (enabled link,
// site file, upstream files) and the generated site rows, then tests and
// reloads nginx once.
func Remove(ctx context.Context, items []*model.Ingress) error {
	res := runBatch(ctx, nil, items, nil, false)
	var failures []string
	for _, ing := range items {
		if r := res.removes[ing.Name]; r.failed {
			failures = append(failures, ing.Name+": "+r.Error)
		}
	}
	if len(failures) > 0 {
		return cosy.WrapErrorWithParams(ErrApplyFailed, strings.Join(failures, "; "))
	}
	return nil
}

// resolveCerts returns the certificate state of every item, asking the
// certificate manager for the ones missing from known.
func resolveCerts(ctx context.Context, items []*model.Ingress, known map[string]CertState) map[string]CertState {
	states := make(map[string]CertState, len(items))
	for _, ing := range items {
		if state, ok := known[ing.Name]; ok {
			states[ing.Name] = state
			continue
		}
		states[ing.Name] = resolveCert(ctx, ing)
	}
	return states
}

// resolveCert asks the certificate manager for the certificate of ing. A
// resolve error is reported as a certificate that is not ready.
func resolveCert(ctx context.Context, ing *model.Ingress) CertState {
	state, err := certs().Resolve(ctx, ing)
	if err != nil {
		state.Ready = false
		if state.Reason == "" {
			state.Reason = "CertNotFound"
		}
		if state.Message == "" {
			state.Message = err.Error()
		}
	}
	return state
}

// runBatch applies and removes ingresses in one transaction with at most one
// test and reload. Items that cannot be prepared (render errors, conflicts)
// are left out of the transaction and reported on their own.
func runBatch(ctx context.Context, applies, removes []*model.Ingress, certStates map[string]CertState, dryRun bool) *batchResult {
	res := &batchResult{
		applies: make(map[string]*itemResult, len(applies)),
		removes: make(map[string]*itemResult, len(removes)),
	}

	prepared := make([]*preparedItem, 0, len(applies))
	for _, ing := range applies {
		item := prepareApply(ing, certStates[ing.Name])
		res.applies[ing.Name] = item.result
		if !item.result.failed {
			prepared = append(prepared, item)
		}
	}
	removing := make([]*preparedItem, 0, len(removes))
	for _, ing := range removes {
		item := prepareRemove(ing)
		res.removes[ing.Name] = item.result
		if !item.result.failed {
			removing = append(removing, item)
		}
	}

	release := config.LockApply()
	defer release()

	// Ownership is checked under the lock so a concurrent site save cannot
	// create the file between the check and the write.
	active := prepared[:0]
	for _, item := range prepared {
		if err := checkSiteOwnership(item.ing.Name, item.sitePath); err != nil {
			item.result.fail(applyReasonRenderFailed, err)
			continue
		}
		active = append(active, item)
	}
	prepared = active
	if len(prepared) == 0 && len(removing) == 0 {
		return res
	}

	claimed := claimedUpstreams()
	tx := &fileTxn{}
	var txErr error
	for _, item := range prepared {
		if err := writeItem(tx, item, claimed, dryRun); err != nil {
			txErr = err
			break
		}
	}
	if txErr == nil {
		for _, item := range removing {
			if err := removeItem(tx, item, claimed, dryRun); err != nil {
				txErr = err
				break
			}
		}
	}
	if txErr != nil {
		txErr = config.RollbackError(txErr, tx.rollback)
		failAll(res, prepared, removing, applyReasonRenderFailed, txErr.Error(), "")
		res.txFailed = true
		res.txReason = applyReasonRenderFailed
		return res
	}

	if !tx.changed() && !dryRun {
		finishApplied(prepared)
		finishRemoved(removing)
		return res
	}

	test := nginx.Control(nginx.TestConfig)
	if dryRun {
		if err := tx.rollback(); err != nil {
			logger.Errorf("Rolling back an ingress dry run failed: %v", err)
		}
		for _, item := range prepared {
			item.result.TestOutput = test.GetOutput()
			if test.IsError() {
				item.result.fail(applyReasonTestFailed, test.GetError())
			}
		}
		return res
	}
	if test.IsError() {
		err := config.RollbackError(test.GetError(), tx.rollback)
		failAll(res, prepared, removing, applyReasonTestFailed, err.Error(), test.GetOutput())
		res.txFailed = true
		res.txReason = applyReasonTestFailed
		return res
	}

	if reload := nginx.Control(nginx.Reload); reload.IsError() {
		err := config.RollbackError(reload.GetError(), func() error {
			if err := tx.rollback(); err != nil {
				return err
			}
			return reloadRestored()
		})
		failAll(res, prepared, removing, applyReasonReloadFailed, err.Error(), reload.GetOutput())
		res.txFailed = true
		res.txReason = applyReasonReloadFailed
		return res
	}

	tx.rescan()
	finishApplied(prepared)
	finishRemoved(removing)
	for _, item := range prepared {
		if item.changed {
			res.changed = append(res.changed, item)
		}
	}
	for _, item := range removing {
		if item.changed && !item.result.failed {
			res.removed = append(res.removed, item)
		}
	}
	return res
}

// reloadRestored loads the configuration a rollback put back in place.
func reloadRestored() error {
	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return fmt.Errorf("test restored configuration: %w", result.GetError())
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		return fmt.Errorf("reload restored configuration: %w", result.GetError())
	}
	return nil
}

func (r *itemResult) fail(reason string, err error) {
	r.failed = true
	r.Reason = reason
	r.Error = err.Error()
}

func failAll(res *batchResult, applies, removes []*preparedItem, reason, message, output string) {
	for _, item := range append(append([]*preparedItem{}, applies...), removes...) {
		item.result.failed = true
		item.result.Reason = reason
		item.result.Error = message
		item.result.TestOutput = output
		item.result.Changed = false
	}
}

func prepareApply(ing *model.Ingress, cert CertState) *preparedItem {
	item := &preparedItem{ing: ing, result: &itemResult{ApplyResult: ApplyResult{Name: ing.Name}}}
	rendered, err := Render(ing, RenderOptions{Cert: cert})
	if err != nil {
		item.result.fail(applyReasonRenderFailed, err)
		return item
	}
	item.rendered = rendered
	item.result.Rendered = rendered

	if err := resolveItemPaths(item, rendered.Upstreams); err != nil {
		item.result.fail(applyReasonRenderFailed, err)
		return item
	}

	namespace, err := loadNamespace(ing.NamespaceID)
	if err != nil {
		item.result.fail(applyReasonRenderFailed, err)
		return item
	}
	item.namespace = namespace
	return item
}

func prepareRemove(ing *model.Ingress) *preparedItem {
	item := &preparedItem{ing: ing, result: &itemResult{ApplyResult: ApplyResult{Name: ing.Name}}}
	upstreams := make(map[string]string, len(ing.Spec.Backends))
	for _, b := range ing.Spec.Backends {
		upstreams[UpstreamName(ing.Name, b.Name)] = ""
	}
	if err := resolveItemPaths(item, upstreams); err != nil {
		item.result.fail(applyReasonRenderFailed, err)
	}
	// The namespace only selects the sync nodes here; a namespace that is
	// gone or deploys remotely must not block the removal.
	if ing.NamespaceID != 0 {
		var namespace model.Namespace
		if err := model.UseDB().Where("id = ?", ing.NamespaceID).First(&namespace).Error; err == nil {
			item.namespace = &namespace
		}
	}
	return item
}

func resolveItemPaths(item *preparedItem, upstreams map[string]string) error {
	name := SiteName(item.ing.Name)
	sitePath, err := config.ResolveConfPathInDir("sites-available", name)
	if err != nil {
		return err
	}
	enabledPath, err := config.ResolveConfPathInDirPreserveLeaf("sites-enabled", name)
	if err != nil {
		return err
	}
	maintenancePath, err := site.ResolveEnabledMaintenancePath(name)
	if err != nil {
		return err
	}
	item.sitePath = sitePath
	item.enabledPath = nginx.GetConfSymlinkPath(enabledPath)
	item.maintenancePath = maintenancePath
	item.upstreams = make(map[string]string, len(upstreams))
	for upstreamName := range upstreams {
		path, err := managed.Path(upstreamName)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrApplyFailed, "upstream "+upstreamName+": "+err.Error())
		}
		item.upstreams[upstreamName] = path
	}
	return nil
}

// loadNamespace returns the namespace of an ingress and refuses namespaces
// that deploy to remote nodes only: ingresses are applied locally.
func loadNamespace(id uint64) (*model.Namespace, error) {
	if id == 0 {
		return nil, nil
	}
	var namespace model.Namespace
	if err := model.UseDB().Where("id = ?", id).First(&namespace).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, cosy.WrapErrorWithParams(ErrApplyFailed, fmt.Sprintf("namespace %d not found", id))
		}
		return nil, err
	}
	if namespace.IsRemoteDeploy() {
		return nil, cosy.WrapErrorWithParams(ErrApplyFailed,
			fmt.Sprintf("namespace %s deploys to remote nodes only; ingresses are applied on this node", namespace.Name))
	}
	return &namespace, nil
}

// checkSiteOwnership refuses to overwrite a site file that does not belong
// to the ingress. A file is ours when its site row is managed by the ingress,
// or, when the row is missing (a crash between the write and the row
// upsert), when it starts with the ingress marker.
func checkSiteOwnership(name, sitePath string) error {
	exists, err := nginx.EntryExists(sitePath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	var site model.Site
	err = model.UseDB().Where("path = ?", sitePath).First(&site).Error
	switch {
	case err == nil:
		if site.ManagedBy == ManagedBy(name) {
			return nil
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		content, readErr := nginx.ReadFile(sitePath)
		if readErr == nil && strings.HasPrefix(string(content), siteHeaderPrefix+name+"\n") {
			return nil
		}
	default:
		return err
	}
	return cosy.WrapErrorWithParams(ErrSiteConflict, SiteName(name))
}

// claimedUpstreams returns every upstream name the stored ingresses render,
// so cleaning up an ingress never deletes a file another ingress owns
// (UpstreamName("a", "b-c") equals UpstreamName("a-b", "c")).
func claimedUpstreams() map[string]string {
	claimed := make(map[string]string)
	var all []*model.Ingress
	if err := model.UseDB().Select("name", "spec").Find(&all).Error; err != nil {
		logger.Errorf("Loading ingresses for upstream cleanup failed: %v", err)
		return claimed
	}
	for _, ing := range all {
		for _, b := range ing.Spec.Backends {
			claimed[UpstreamName(ing.Name, b.Name)] = ing.Name
		}
	}
	return claimed
}

// reUpstreamReference finds the upstreams a previous rendering generated:
// the upstreams marker line and the proxy_pass targets.
var (
	reUpstreamsLine     = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(upstreamsMarker) + `(.*)$`)
	reUpstreamReference = regexp.MustCompile(`proxy_pass\s+http://(ig-[a-z0-9-]+);`)
)

// staleUpstreams returns the upstreams the site file on disk proxies to that
// the ingress no longer renders (current) and no other ingress claims. The
// names come from the previous rendering, not from a file name glob, because
// UpstreamName prefixes collide across ingresses.
func staleUpstreams(item *preparedItem, current map[string]string, claimed map[string]string) map[string]string {
	stale := make(map[string]string)
	content, err := nginx.ReadFile(item.sitePath)
	if err != nil {
		return stale
	}
	var previous []string
	if m := reUpstreamsLine.FindStringSubmatch(string(content)); m != nil {
		previous = strings.Fields(m[1])
	}
	for _, m := range reUpstreamReference.FindAllStringSubmatch(string(content), -1) {
		previous = append(previous, m[1])
	}
	prefix := "ig-" + item.ing.Name + "-"
	for _, upstreamName := range previous {
		if !strings.HasPrefix(upstreamName, prefix) || !ValidateBackendName(strings.TrimPrefix(upstreamName, prefix)) {
			continue
		}
		if _, ok := current[upstreamName]; ok {
			continue
		}
		if owner, ok := claimed[upstreamName]; ok && owner != item.ing.Name {
			continue
		}
		if path, err := managed.Path(upstreamName); err == nil {
			stale[upstreamName] = path
		}
	}
	return stale
}

func writeItem(tx *fileTxn, item *preparedItem, claimed map[string]string, dryRun bool) error {
	for _, dir := range []string{filepath.Dir(item.sitePath), filepath.Dir(item.enabledPath), managed.Dir()} {
		if err := nginx.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	before := tx.changes
	// Read the previous rendering before the site file is replaced.
	stale := staleUpstreams(item, item.rendered.Upstreams, claimed)
	for _, upstreamName := range sortedKeys(item.rendered.Upstreams) {
		path := item.upstreams[upstreamName]
		if err := tx.write(path, item.rendered.Upstreams[upstreamName], !dryRun); err != nil {
			return err
		}
	}
	if err := tx.write(item.sitePath, item.rendered.SiteContent, !dryRun); err != nil {
		return err
	}
	if err := tx.link(item.sitePath, item.enabledPath); err != nil {
		return err
	}
	// A forced maintenance switch must not stay enabled next to the site.
	if err := tx.remove(item.maintenancePath, !dryRun); err != nil {
		return err
	}
	for _, upstreamName := range sortedKeys(stale) {
		if err := tx.remove(stale[upstreamName], !dryRun); err != nil {
			return err
		}
	}
	item.changed = tx.changes > before
	item.result.Changed = item.changed
	return nil
}

func removeItem(tx *fileTxn, item *preparedItem, claimed map[string]string, dryRun bool) error {
	before := tx.changes
	if err := tx.unlink(item.sitePath, item.enabledPath); err != nil {
		return err
	}
	if err := tx.remove(item.maintenancePath, !dryRun); err != nil {
		return err
	}
	upstreams := staleUpstreams(item, nil, claimed)
	if err := tx.remove(item.sitePath, !dryRun); err != nil {
		return err
	}
	for upstreamName, path := range item.upstreams {
		if owner, ok := claimed[upstreamName]; ok && owner != item.ing.Name {
			continue
		}
		upstreams[upstreamName] = path
	}
	for _, upstreamName := range sortedKeys(upstreams) {
		if err := tx.remove(upstreams[upstreamName], !dryRun); err != nil {
			return err
		}
	}
	item.changed = tx.changes > before
	item.result.Changed = item.changed
	return nil
}

// finishApplied records the generated site rows of the applied items.
func finishApplied(items []*preparedItem) {
	for _, item := range items {
		if err := upsertSiteRow(item); err != nil {
			item.result.fail(applyReasonRenderFailed, fmt.Errorf("record site: %w", err))
			continue
		}
		item.result.Reason = applyReasonApplied
	}
}

// finishRemoved deletes the generated site rows of the removed items.
func finishRemoved(items []*preparedItem) {
	for _, item := range items {
		err := model.UseDB().Unscoped().
			Where("path = ? AND managed_by = ?", item.sitePath, ManagedBy(item.ing.Name)).
			Delete(&model.Site{}).Error
		if err != nil {
			item.result.fail(applyReasonRenderFailed, fmt.Errorf("delete site row: %w", err))
		}
	}
}

func upsertSiteRow(item *preparedItem) error {
	db := model.UseDB()
	var site model.Site
	err := db.Unscoped().Where("path = ?", item.sitePath).First(&site).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&model.Site{
			Path:        item.sitePath,
			NamespaceID: item.ing.NamespaceID,
			ManagedBy:   ManagedBy(item.ing.Name),
		}).Error
	}
	if err != nil {
		return err
	}
	return db.Unscoped().Model(&model.Site{}).Where("id = ?", site.ID).Updates(map[string]any{
		"deleted_at":   nil,
		"namespace_id": item.ing.NamespaceID,
		"managed_by":   ManagedBy(item.ing.Name),
	}).Error
}

// fileTxn records how to undo every file change of a batch.
type fileTxn struct {
	undo []func() error
	// changes counts the operations that changed the disk.
	changes int
	// scans lists the files whose proxy targets the upstream service must
	// learn again after a successful reload; nil content means removed.
	scans []fileScan
}

type fileScan struct {
	path    string
	content []byte
}

func (t *fileTxn) changed() bool {
	return t.changes > 0
}

// write replaces path with content unless it already holds it. history
// keeps a config backup of the previous content.
func (t *fileTxn) write(path, content string, history bool) error {
	current, err := nginx.ReadFile(path)
	if err == nil && string(current) == content {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	snapshot, err := config.CaptureFile(path)
	if err != nil {
		return err
	}
	if history {
		if err := config.CheckAndCreateHistory(path, content); err != nil {
			return err
		}
	}
	t.undo = append(t.undo, func() error { return snapshot.Restore(path) })
	t.changes++
	t.scans = append(t.scans, fileScan{path: path, content: []byte(content)})
	return config.WriteFile(path, []byte(content), 0o644)
}

// remove deletes path when it exists.
func (t *fileTxn) remove(path string, history bool) error {
	exists, err := nginx.EntryExists(path)
	if err != nil || !exists {
		return err
	}
	snapshot, err := config.CaptureFile(path)
	if err != nil {
		return err
	}
	if history {
		if err := config.CheckAndCreateHistory(path, ""); err != nil {
			return err
		}
	}
	t.undo = append(t.undo, func() error { return snapshot.Restore(path) })
	t.changes++
	t.scans = append(t.scans, fileScan{path: path})
	return nginx.Remove(path)
}

// link creates the sites-enabled symlink unless an entry is already there.
func (t *fileTxn) link(target, linkPath string) error {
	exists, err := nginx.EntryExists(linkPath)
	if err != nil || exists {
		return err
	}
	if err := nginx.Symlink(target, linkPath); err != nil {
		return err
	}
	t.undo = append(t.undo, func() error { return config.RemoveEnabledLink(linkPath) })
	t.changes++
	return nil
}

// unlink removes the sites-enabled entry; rollback links it to target again.
func (t *fileTxn) unlink(target, linkPath string) error {
	exists, err := nginx.EntryExists(linkPath)
	if err != nil || !exists {
		return err
	}
	isLink, err := nginx.SymlinkExists(linkPath)
	if err != nil {
		return err
	}
	if !isLink {
		// A regular file in sites-enabled is restored like any other file.
		return t.remove(linkPath, false)
	}
	if err := config.RemoveEnabledLink(linkPath); err != nil {
		return err
	}
	t.undo = append(t.undo, func() error { return nginx.Symlink(target, linkPath) })
	t.changes++
	return nil
}

// rollback undoes every change in reverse order.
func (t *fileTxn) rollback() error {
	errs := make([]error, 0)
	for i := len(t.undo) - 1; i >= 0; i-- {
		if err := t.undo[i](); err != nil {
			errs = append(errs, err)
		}
	}
	t.undo = nil
	return errors.Join(errs...)
}

// rescan tells the upstream availability service about the changed files
// right away instead of waiting for the file watcher.
func (t *fileTxn) rescan() {
	sort.SliceStable(t.scans, func(i, j int) bool { return t.scans[i].path < t.scans[j].path })
	for _, scan := range t.scans {
		if err := upstream.ScanConfig(scan.path, scan.content); err != nil {
			logger.Debugf("Rescanning %s failed: %v", scan.path, err)
		}
	}
}
