package template

import (
	"io/fs"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

var (
	e = cosy.NewErrorScope("template")

	// ErrTemplateNotFound is returned for a template no enabled plugin offers.
	ErrTemplateNotFound = e.New(56001, "template not found")
)

// Root is a directory of templates laid out like the built-in ones, with
// conf/*.conf and block/*.conf below it.
type Root struct {
	PluginID string
	// Dir is the absolute path of the directory.
	Dir string
}

// Source offers template directories that come and go at runtime, such as
// the templates of content plugins.
type Source interface {
	// TemplateRoots lists the directories the source offers right now.
	TemplateRoots() []Root
}

var (
	sources      []Source
	sourcesMutex sync.RWMutex
)

// RegisterSource adds a source of template directories.
func RegisterSource(source Source) {
	sourcesMutex.Lock()
	defer sourcesMutex.Unlock()
	sources = append(sources, source)
}

// roots lists the directories of every source, ordered by plugin id. The
// first directory of a plugin id wins.
func roots() []Root {
	sourcesMutex.RLock()
	registered := append([]Source(nil), sources...)
	sourcesMutex.RUnlock()

	var all []Root
	seen := map[string]struct{}{}
	for _, source := range registered {
		for _, root := range source.TemplateRoots() {
			if root.PluginID == "" || root.Dir == "" {
				continue
			}
			if _, dup := seen[root.PluginID]; dup {
				continue
			}
			seen[root.PluginID] = struct{}{}
			all = append(all, root)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].PluginID < all[j].PluginID })
	return all
}

// pluginLocation resolves the templates of an enabled plugin.
func pluginLocation(pluginID string) (location, error) {
	for _, root := range roots() {
		if root.PluginID == pluginID {
			return location{fsys: os.DirFS(root.Dir), origin: OriginPlugin, pluginID: pluginID}, nil
		}
	}
	return location{}, ErrTemplateNotFound
}

// checkPluginFile makes sure a plugin template is a regular file in a real
// kind directory, so a symlink cannot make the host read a file outside the
// plugin.
func checkPluginFile(fsys fs.FS, kind, name string) error {
	info, err := fs.Lstat(fsys, kind)
	if err != nil || !info.IsDir() {
		return ErrTemplateNotFound
	}
	info, err = fs.Lstat(fsys, path.Join(kind, name))
	if err != nil || !info.Mode().IsRegular() {
		return ErrTemplateNotFound
	}
	return nil
}

// pluginTemplateList lists the templates of a kind every enabled plugin
// contributes. A template that does not parse is skipped, install time
// validation keeps them out in the first place.
func pluginTemplateList(kind string) []ConfigInfoItem {
	var list []ConfigInfoItem
	for _, root := range roots() {
		loc := location{fsys: os.DirFS(root.Dir), origin: OriginPlugin, pluginID: root.PluginID}
		if info, err := fs.Lstat(loc.fsys, kind); err != nil || !info.IsDir() {
			continue
		}
		for _, name := range templateFiles(loc.fsys, kind) {
			info, err := readInfo(loc, kind, name)
			if err != nil {
				logger.Warnf("[plugin:%s] skip template %s/%s: %v", root.PluginID, kind, name, err)
				continue
			}
			list = append(list, info)
		}
	}
	return list
}

// templateFiles lists the template file names of one kind directory,
// ignoring anything that is not a regular file with a valid name.
func templateFiles(fsys fs.FS, kind string) []string {
	entries, err := fs.ReadDir(fsys, kind)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !IsValidFileName(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}
