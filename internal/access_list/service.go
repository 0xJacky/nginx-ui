package access_list

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
)

// testAndReload validates and loads the configuration after the rendered files
// changed. Tests replace it to exercise the rollback path without Nginx.
var testAndReload = func(tx *config.FileTransaction) error {
	return tx.TestAndReload()
}

// rollbackAndReload restores the files and reloads Nginx after the database
// rejected a change Nginx already loaded. Tests replace it as well.
var rollbackAndReload = func(tx *config.FileTransaction) error {
	return tx.RollbackAndReload()
}

// SaveResult reports what a save touched.
type SaveResult struct {
	List *model.AccessList `json:"list"`
	// Slugs holds the list itself and every list that references it, whose
	// files were rendered again.
	Slugs []string `json:"slugs"`
	// References are the sites and streams that include one of Slugs.
	References []Reference `json:"references"`
}

// All returns every stored access list.
func All() ([]*model.AccessList, error) {
	a := query.AccessList
	return a.Order(a.Name).Find()
}

// Get returns one access list.
func Get(id uint64) (*model.AccessList, error) {
	a := query.AccessList
	list, err := a.Where(a.ID.Eq(id)).First()
	if err != nil {
		return nil, ErrAccessListNotFound
	}
	return list, nil
}

// Preview renders a list that has not been saved yet.
func Preview(list *model.AccessList) (content string, warnings []Warning, err error) {
	lists, err := All()
	if err != nil {
		return "", nil, err
	}
	others := withoutID(lists, list.ID)
	if list.Slug == "" {
		list.Slug = "preview"
	}
	if err = Validate(list, others); err != nil {
		return "", nil, err
	}
	content, err = Render(list, others)
	if err != nil {
		return "", nil, err
	}
	return content, Warnings(list), nil
}

// Save creates or updates a list, renders its file and the files of every
// list that references it, and reloads Nginx when a site or stream uses one of
// them. The database is written only after Nginx accepted the files, so its
// write lock is never held across `nginx -t` and the reload: on SQLite every
// other writer would fail with "database is locked" once those outlast the
// busy timeout. A rejected test leaves the database untouched, and a failed
// database write restores the files and the running configuration.
func Save(list *model.AccessList) (*SaveResult, error) {
	lists, err := All()
	if err != nil {
		return nil, err
	}

	if list.ID != 0 {
		var stored *model.AccessList
		for _, l := range lists {
			if l.ID == list.ID {
				stored = l
			}
		}
		if stored == nil {
			return nil, ErrAccessListNotFound
		}
		// The slug names the file every include points at; it never changes.
		list.Slug = stored.Slug
		list.CreatedAt = stored.CreatedAt
	}
	list.Name = strings.TrimSpace(list.Name)
	if list.Fallback == "" {
		list.Fallback = model.AccessFallbackDeny
	}

	others := withoutID(lists, list.ID)
	if err = Validate(list, others); err != nil {
		return nil, err
	}

	combined := append(append([]*model.AccessList{}, others...), list)
	affected := []*model.AccessList{list}
	if list.ID != 0 {
		affected = append(affected, Dependents(list.ID, combined)...)
	}

	rendered := make(map[string]string, len(affected))
	slugs := make([]string, 0, len(affected))
	for _, l := range affected {
		content, err := Render(l, combined)
		if err != nil {
			return nil, err
		}
		rendered[l.Slug] = content
		slugs = append(slugs, l.Slug)
	}

	release := config.LockApply()
	defer release()

	refs := FilterReferences(ScanReferences(), slugs...)

	inUse := len(refs) > 0
	files, err := writeFiles(rendered, inUse)
	if err != nil {
		return nil, err
	}

	if err = model.UseDB().Save(list).Error; err != nil {
		rollback := files.Rollback
		if inUse {
			rollback = func() error { return rollbackAndReload(files) }
		}
		return nil, config.RollbackError(err, rollback)
	}

	return &SaveResult{List: list, Slugs: slugs, References: refs}, nil
}

// writeFiles renders the files and, when anything includes them, tests and
// reloads Nginx. A file nothing includes is never loaded, so testing it would
// only let an unrelated broken site block the save. On success it returns the
// transaction so the caller can still undo the files.
func writeFiles(rendered map[string]string, inUse bool) (*config.FileTransaction, error) {
	if err := nginx.MkdirAll(Dir(), 0o755); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrWriteAccessListFile, err.Error())
	}

	tx := &config.FileTransaction{}
	for slug, content := range rendered {
		if err := tx.Write(FilePath(slug), []byte(content), 0o644); err != nil {
			return nil, config.RollbackError(cosy.WrapErrorWithParams(ErrWriteAccessListFile, err.Error()), tx.Rollback)
		}
	}
	if !inUse {
		return tx, nil
	}
	if err := testAndReload(tx); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrNginxTestFailed, err.Error())
	}
	return tx, nil
}

// Delete removes a list and its file. A list that another list, a site or a
// stream still uses is kept.
func Delete(id uint64) error {
	list, err := Get(id)
	if err != nil {
		return err
	}
	lists, err := All()
	if err != nil {
		return err
	}

	release := config.LockApply()
	defer release()

	dependents := DirectDependents(id, lists)
	refs := FilterReferences(ScanReferences(), list.Slug)
	if len(dependents) > 0 || len(refs) > 0 {
		names := make([]string, 0, len(dependents))
		for _, l := range dependents {
			names = append(names, l.Name)
		}
		return cosy.WrapErrorWithParams(ErrAccessListInUse, describeReferences(refs, names))
	}

	// Nothing includes the file, so removing it needs no reload. It goes
	// first and comes back when the row cannot be deleted, which keeps the
	// database write out of the file operation.
	path := FilePath(list.Slug)
	snapshot, err := config.CaptureFile(path)
	if err != nil {
		return err
	}
	if err = nginx.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = model.UseDB().Unscoped().Delete(&model.AccessList{}, id).Error; err != nil {
		return config.RollbackError(err, func() error { return snapshot.Restore(path) })
	}
	return nil
}

// Usage describes who uses a list.
type Usage struct {
	Lists      []*model.AccessList `json:"lists"`
	References []Reference         `json:"references"`
}

// UsageOf returns the lists that reference a list directly and the site and
// stream includes of it.
func UsageOf(id uint64) (*Usage, error) {
	list, err := Get(id)
	if err != nil {
		return nil, err
	}
	lists, err := All()
	if err != nil {
		return nil, err
	}
	dependents := DirectDependents(id, lists)
	if dependents == nil {
		dependents = []*model.AccessList{}
	}
	refs := FilterReferences(ScanReferences(), list.Slug)
	if refs == nil {
		refs = []Reference{}
	}
	return &Usage{Lists: dependents, References: refs}, nil
}

// SlugsExist reports the first slug that has no stored list.
func SlugsExist(slugs []string) error {
	if len(slugs) == 0 {
		return nil
	}
	a := query.AccessList
	stored, err := a.Where(a.Slug.In(slugs...)).Find()
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for _, l := range stored {
		found[l.Slug] = true
	}
	for _, slug := range slugs {
		if !found[slug] {
			return cosy.WrapErrorWithParams(ErrUnknownList, slug)
		}
	}
	return nil
}

// ReadFiles returns the rendered files of the given lists, keyed by their path
// relative to the Nginx configuration directory. Missing files are skipped.
func ReadFiles(slugs []string) map[string]string {
	files := make(map[string]string, len(slugs))
	for _, slug := range slugs {
		if !ValidSlug(slug) {
			continue
		}
		content, err := nginx.ReadFile(FilePath(slug))
		if err != nil {
			continue
		}
		files[filepath.ToSlash(IncludePath(slug))] = string(content)
	}
	return files
}

func withoutID(lists []*model.AccessList, id uint64) []*model.AccessList {
	result := make([]*model.AccessList, 0, len(lists))
	for _, l := range lists {
		if id == 0 || l.ID != id {
			result = append(result, l)
		}
	}
	return result
}
