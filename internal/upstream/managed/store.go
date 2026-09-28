package managed

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	filePrefix = "upstream-"
	fileSuffix = ".conf"
)

// Reference types reported by FindReferences.
const (
	ReferenceSite   = "site"
	ReferenceConfig = "config"
)

// Reference is a configuration file that proxies to an upstream by name.
type Reference struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Detail is a managed upstream together with where it lives and who uses it.
type Detail struct {
	Upstream
	Path       string      `json:"path"`
	Content    string      `json:"content"`
	References []Reference `json:"references"`
}

// FileName returns the file name a managed upstream is stored under.
func FileName(name string) string {
	return filePrefix + name + fileSuffix
}

// Dir returns the directory managed upstream files are written to.
func Dir() string {
	return nginx.GetConfPath("conf.d")
}

// Path returns the absolute path of the managed upstream file for name. The
// name comes from the request URL or body, so it is checked by the name
// pattern and then confined to conf.d by confinedPath.
func Path(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return confinedPath(name)
}

// confinedPath builds the managed file path for name and proves it cannot
// leave conf.d. It does not rely on ValidateName: path separators and ".."
// are rejected explicitly, and the joined path is resolved through
// config.ResolveConfPathInDir, which refuses anything outside the directory
// (following symlinks), the same boundary the site and config editors use.
func confinedPath(name string) (string, error) {
	if name == "" || strings.Contains(name, "..") {
		return "", ErrInvalidName
	}
	if strings.ContainsAny(name, `/\`) || !filepath.IsLocal(name) {
		return "", ErrInvalidName
	}

	path, err := config.ResolveConfPathInDir("conf.d", FileName(name))
	if err != nil {
		return "", err
	}

	dir := filepath.Clean(Dir())
	path = filepath.Clean(path)
	if filepath.Dir(path) != dir || !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
		return "", cosy.WrapErrorWithParams(config.ErrPathIsNotUnderTheNginxConfDir, path, dir)
	}
	return path, nil
}

// nameFromFile extracts the upstream name from a managed file name.
func nameFromFile(fileName string) (string, bool) {
	if !strings.HasPrefix(fileName, filePrefix) || !strings.HasSuffix(fileName, fileSuffix) {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(fileName, filePrefix), fileSuffix)
	if ValidateName(name) != nil {
		return "", false
	}
	return name, true
}

// Read loads and parses the managed upstream called name.
func Read(name string) (*Upstream, string, string, error) {
	path, err := Path(name)
	if err != nil {
		return nil, "", "", err
	}
	content, err := nginx.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, "", "", cosy.WrapErrorWithParams(ErrUpstreamNotFound, name)
	}
	if err != nil {
		return nil, "", "", err
	}
	u, err := Parse(name, string(content))
	if err != nil {
		return nil, "", "", err
	}
	return u, path, string(content), nil
}

// Exists reports whether a managed upstream called name is on disk.
func Exists(name string) bool {
	path, err := Path(name)
	if err != nil {
		return false
	}
	ok, err := nginx.Exists(path)
	return err == nil && ok
}

// List returns every managed upstream sorted by name. Files that follow the
// naming scheme but were edited into something Render cannot produce are
// skipped; they still show up as regular upstream definitions.
func List() ([]*Detail, error) {
	entries, err := nginx.ReadDir(Dir())
	if os.IsNotExist(err) {
		return []*Detail{}, nil
	}
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrConfDirUnavailable, err.Error())
	}

	idx := newReferenceIndex()
	result := make([]*Detail, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name, ok := nameFromFile(entry.Name())
		if !ok {
			continue
		}
		u, path, content, err := Read(name)
		if err != nil {
			logger.Debugf("skip upstream file %s: %v", entry.Name(), err)
			continue
		}
		result = append(result, &Detail{
			Upstream:   *u,
			Path:       path,
			Content:    content,
			References: idx.find(name, path),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// Get returns one managed upstream with its references.
func Get(name string) (*Detail, error) {
	u, path, content, err := Read(name)
	if err != nil {
		return nil, err
	}
	refs, err := FindReferences(name)
	if err != nil {
		return nil, err
	}
	return &Detail{Upstream: *u, Path: path, Content: content, References: refs}, nil
}

// Prepare normalizes and validates u and returns the file content it renders to.
func Prepare(u *Upstream) (string, error) {
	u.Normalize()
	if err := u.Validate(); err != nil {
		return "", err
	}
	return u.Render(), nil
}

// definedElsewhere returns the config file that already defines an upstream
// called name, other than the managed file itself. nginx rejects duplicate
// upstream names, so this turns a cryptic `nginx -t` failure into a clear one.
func definedElsewhere(name, managedPath string) string {
	def, ok := upstream.GetUpstreamService().GetUpstreamDefinition(name)
	if !ok || def.ConfigPath == "" || samePath(def.ConfigPath, managedPath) {
		return ""
	}
	// The scanner may still remember a file that was deleted since.
	if exists, err := nginx.Exists(def.ConfigPath); err != nil || !exists {
		return ""
	}
	return def.ConfigPath
}

func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := nginx.EvalSymlinks(a)
	rb, errB := nginx.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// Save writes the upstream, then tests and reloads nginx. A rejected
// configuration is rolled back by config.Save. create selects between adding a
// new upstream and replacing an existing one; renaming is not supported
// because sites reference the group by its name.
func Save(u *Upstream, create bool, userName string) (*Detail, error) {
	content, err := Prepare(u)
	if err != nil {
		return nil, err
	}

	path, err := Path(u.Name)
	if err != nil {
		return nil, err
	}

	exists, err := nginx.Exists(path)
	if err != nil {
		return nil, err
	}
	if create && exists {
		return nil, cosy.WrapErrorWithParams(ErrUpstreamExists, u.Name)
	}
	if !create {
		if !exists {
			return nil, cosy.WrapErrorWithParams(ErrUpstreamNotFound, u.Name)
		}
		// Refuse to overwrite a hand-written file that merely matches the
		// naming scheme.
		if _, _, _, err := Read(u.Name); err != nil {
			return nil, err
		}
	}
	if other := definedElsewhere(u.Name, path); other != "" {
		return nil, cosy.WrapErrorWithParams(ErrUpstreamNameConflict, other)
	}

	if err := nginx.MkdirAll(Dir(), 0o755); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrConfDirUnavailable, err.Error())
	}

	if err := config.Save(path, content, nil, userName); err != nil {
		return nil, explainZoneConflict(u, err)
	}

	// Refresh the availability service right away instead of waiting for the
	// file watcher, so the upstream list and site cards pick up the change.
	if err := upstream.ScanConfig(path, []byte(content)); err != nil {
		logger.Error(err)
	}

	return Get(u.Name)
}

// explainZoneConflict turns the nginx -t failure for a zone name that another
// directive (limit_req_zone, proxy_cache_path, ...) already declared into a
// readable error. config.Save has rolled the file back at this point. Other
// upstream blocks may share the zone, so that case never reaches here.
func explainZoneConflict(u *Upstream, err error) error {
	if !u.Zone {
		return err
	}
	marker := `shared memory zone "` + u.Name + `" is already declared`
	msg := err.Error()
	if !strings.Contains(msg, marker) {
		return err
	}
	for _, line := range strings.Split(msg, "\n") {
		if strings.Contains(line, marker) {
			if i := strings.Index(line, "[emerg] "); i >= 0 {
				line = line[i+len("[emerg] "):]
			}
			return cosy.WrapErrorWithParams(ErrZoneNameConflict, u.Name, strings.TrimSpace(line))
		}
	}
	return err
}

// Delete removes a managed upstream. It refuses while any site or config
// still proxies to it, because nginx would then fail to resolve the name and
// every later `nginx -t` would break.
func Delete(name string) error {
	if _, _, _, err := Read(name); err != nil {
		return err
	}
	path, err := Path(name)
	if err != nil {
		return err
	}

	refs, err := FindReferences(name)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		names := make([]string, 0, len(refs))
		for _, ref := range refs {
			names = append(names, ref.Name)
		}
		return cosy.WrapErrorWithParams(ErrUpstreamInUse, strings.Join(names, ", "))
	}

	release := config.LockApply()
	defer release()

	snapshot, err := config.CaptureFile(path)
	if err != nil {
		return err
	}
	if err := config.CheckAndCreateHistory(path, ""); err != nil {
		return err
	}
	if err := nginx.Remove(path); err != nil {
		return err
	}

	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return config.RollbackError(result.GetError(), func() error {
			return snapshot.Restore(path)
		})
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		return config.RollbackError(result.GetError(), func() error {
			return config.RestoreAndReload(path, snapshot)
		})
	}

	// Mirror the deletion to the nodes the file was synced to, including the
	// ones inherited from a synced conf.d directory, as config.Save did for
	// the writes.
	cfg, cfgErr := query.Config.Where(query.Config.Filepath.Eq(path)).First()
	if cfgErr != nil {
		cfg = &model.Config{Filepath: path}
	}
	syncNodeIDs, _ := config.EffectiveSyncTargets(cfg)
	if err := config.CleanupDatabaseRecords(path, false); err != nil {
		logger.Error(err)
	}
	if len(syncNodeIDs) > 0 {
		if err := config.SyncDeleteOnRemoteServer(path, syncNodeIDs); err != nil {
			logger.Error(err)
		}
	}

	if err := upstream.ScanConfig(path, nil); err != nil {
		logger.Error(err)
	}
	return nil
}

// passDirectives are the directives whose target can name an upstream group.
const passDirectives = `proxy_pass|grpc_pass|fastcgi_pass|uwsgi_pass|scgi_pass|memcached_pass`

// referencePattern matches an uncommented pass directive whose target host is
// exactly name, with an optional scheme, port and URI.
func referencePattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[^#\n]*?(?:^|[\s;{])(?:` + passDirectives + `)\s+` +
		`(?:[A-Za-z][A-Za-z0-9+.-]*://)?` + regexp.QuoteMeta(name) + `(?::\d+)?(?:[/;\s?]|$)`)
}

// referenceFile is one scanned configuration file.
type referenceFile struct {
	ref     Reference
	content string
}

// referenceIndex holds the files that may reference an upstream so listing
// many upstreams reads each file once.
type referenceIndex struct {
	files []referenceFile
}

func newReferenceIndex() *referenceIndex {
	idx := &referenceIndex{}
	idx.load(nginx.GetConfPath("sites-available"), ReferenceSite, nil)
	idx.load(Dir(), ReferenceConfig, func(name string) bool {
		return strings.HasSuffix(name, fileSuffix)
	})
	return idx
}

func (idx *referenceIndex) load(dir, refType string, filter func(string) bool) {
	entries, err := nginx.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || (filter != nil && !filter(entry.Name())) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := nginx.ReadFile(path)
		if err != nil {
			continue
		}
		idx.files = append(idx.files, referenceFile{
			ref:     Reference{Type: refType, Name: entry.Name(), Path: path},
			content: string(content),
		})
	}
}

func (idx *referenceIndex) find(name, selfPath string) []Reference {
	pattern := referencePattern(name)
	refs := make([]Reference, 0)
	for _, f := range idx.files {
		if samePath(f.ref.Path, selfPath) {
			continue
		}
		if pattern.MatchString(f.content) {
			refs = append(refs, f.ref)
		}
	}
	return refs
}

// FindReferences lists the sites and conf.d files that proxy to name.
func FindReferences(name string) ([]Reference, error) {
	path, err := Path(name)
	if err != nil {
		return nil, err
	}
	return newReferenceIndex().find(name, path), nil
}

// Names returns the names of all managed upstreams.
func Names() []string {
	list, err := List()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, item := range list {
		names = append(names, item.Name)
	}
	return names
}
