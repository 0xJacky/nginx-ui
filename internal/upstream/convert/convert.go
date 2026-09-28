// Package convert turns an upstream block written inside a site file into a
// managed upstream group (conf.d/upstream-<name>.conf).
//
// The group keeps its name, so every `proxy_pass http://<name>` of the site
// keeps working, and other sites can pick it from then on. The new group file
// and the site without the block are written together and checked with a
// single `nginx -t`; when nginx rejects the result or does not reload, both
// files are put back exactly as they were.
package convert

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/internal/upstream/serverstate"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gen/field"
)

// Request names the upstream block to convert and the site that holds it.
type Request struct {
	Site     string `json:"site"`
	Upstream string `json:"upstream"`
	// Zone gives a block that declares no zone the shared memory zone new
	// groups get (`zone <name> <size>;`), so every worker process balances
	// with the same state. Omitted means on; a block with a zone directive of
	// its own keeps it either way.
	Zone *bool `json:"zone,omitempty"`
	// ZoneSize is the size of that zone; empty means the default size.
	ZoneSize string `json:"zone_size,omitempty"`
}

// zoneEnabled reports whether the request wants a shared memory zone.
func (r Request) zoneEnabled() bool {
	return r.Zone == nil || *r.Zone
}

// plan is a validated conversion that has not touched the disk yet.
type plan struct {
	siteName       string
	sitePath       string
	managedPath    string
	managedContent string
	siteContent    string
	// group is the upstream the group file renders, used to explain an
	// nginx -t failure about its zone.
	group *managed.Upstream
	// mirror is the request sent to the nodes that receive the site, with the
	// zone choice spelled out.
	mirror Request
}

// Convert moves the upstream block req.Upstream out of the site req.Site into
// a managed upstream group, then tests and reloads nginx once. It returns the
// new group.
func Convert(ctx context.Context, req Request, userName string) (*managed.Detail, error) {
	// Hold the apply lock from reading the site to reloading nginx, so no
	// other save can change either file in between.
	release := config.LockApply()
	defer release()

	p, err := prepare(req)
	if err != nil {
		return nil, err
	}

	if err := apply(p); err != nil {
		// Both files are back at this point; a zone name that another
		// directive already uses gets the same readable error as a group
		// saved from the Upstream Groups page.
		return nil, managed.ExplainZoneConflict(p.group, err)
	}

	cfg := managedConfigRecord(p.managedPath)
	rescan(p)
	startSync(ctx, p, cfg, userName)

	return managed.Get(p.group.Name)
}

// prepare reads the site, locates the block and renders both new files.
func prepare(req Request) (*plan, error) {
	siteName := strings.TrimSpace(req.Site)
	name := strings.TrimSpace(req.Upstream)
	if siteName == "" || name == "" {
		return nil, ErrInvalidRequest
	}
	if err := managed.ValidateName(name); err != nil {
		return nil, err
	}

	sitePath, err := resolveSitePath(siteName)
	if err != nil {
		return nil, err
	}

	raw, err := nginx.ReadFile(sitePath)
	if os.IsNotExist(err) {
		return nil, cosy.WrapErrorWithParams(ErrSiteNotFound, siteName)
	}
	if err != nil {
		return nil, err
	}
	original := string(raw)

	managedPath, err := managed.Path(name)
	if err != nil {
		return nil, err
	}
	if exists, err := nginx.Exists(managedPath); err != nil {
		return nil, err
	} else if exists {
		return nil, cosy.WrapErrorWithParams(ErrGroupExists, name)
	}

	block, err := findBlock(original, name, sitePath)
	if err != nil {
		return nil, err
	}
	if other := definedElsewhere(name, sitePath); other != "" {
		return nil, cosy.WrapErrorWithParams(managed.ErrUpstreamNameConflict, other)
	}

	u, err := toUpstream(name, block)
	if err != nil {
		return nil, err
	}
	applyZoneChoice(u, req)
	managedContent, err := managed.Prepare(u)
	if err != nil {
		return nil, err
	}

	siteContent := RemoveBlock(original, block.Start, block.End)

	if err := config.ValidateConfigFile(managedPath, managedContent); err != nil {
		return nil, err
	}
	if err := config.ValidateConfigFile(sitePath, siteContent); err != nil {
		return nil, err
	}

	return &plan{
		siteName:       siteName,
		sitePath:       sitePath,
		managedPath:    managedPath,
		managedContent: managedContent,
		siteContent:    siteContent,
		group:          u,
		mirror:         mirrorRequest(siteName, name, req),
	}, nil
}

// applyZoneChoice gives a block without a zone directive the shared memory
// zone new groups get, unless the request turned it off. A block that already
// has one keeps it as it is: its own zone is in the zone fields, and a zone of
// another name, or `zone other;` joining a zone declared elsewhere, stays in
// the additional directives, since an upstream holds a single zone.
func applyZoneChoice(u *managed.Upstream, req Request) {
	if u.Zone || managed.HasZoneDirective(u.ExtraDirectives) || !req.zoneEnabled() {
		return
	}
	u.Zone = true
	// Normalize fills in the default size when this is empty.
	u.ZoneSize = strings.TrimSpace(req.ZoneSize)
}

// mirrorRequest is the request replayed on the nodes that receive the site.
// The zone choice is always explicit, so a node converts the same way
// whatever its own default is.
func mirrorRequest(siteName, name string, req Request) Request {
	zone := req.zoneEnabled()
	mirror := Request{Site: siteName, Upstream: name, Zone: &zone}
	if zone {
		mirror.ZoneSize = strings.TrimSpace(req.ZoneSize)
	}
	return mirror
}

// resolveSitePath maps a site name onto its file in sites-available and
// refuses anything that is not a plain file name there.
func resolveSitePath(name string) (string, error) {
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) || !filepath.IsLocal(name) {
		return "", cosy.WrapErrorWithParams(ErrInvalidSiteName, name)
	}
	path, err := site.ResolveAvailablePath(name)
	if err != nil {
		return "", err
	}
	dir := nginx.GetConfPath("sites-available")
	if !helper.IsUnderDirectory(path, dir) || filepath.Dir(filepath.Clean(path)) != filepath.Clean(dir) {
		return "", cosy.WrapErrorWithParams(config.ErrPathIsNotUnderTheNginxConfDir, path, dir)
	}
	return path, nil
}

// findBlock returns the only upstream block called name in content.
func findBlock(content, name, path string) (serverstate.Block, error) {
	var found []serverstate.Block
	for _, block := range serverstate.ParseBlocks(content) {
		if block.Name == name && block.End > 0 {
			found = append(found, block)
		}
	}
	switch len(found) {
	case 0:
		return serverstate.Block{}, cosy.WrapErrorWithParams(serverstate.ErrUpstreamNotInConfig, name, path)
	case 1:
	default:
		return serverstate.Block{}, cosy.WrapErrorWithParams(ErrUpstreamDefinedTwice, name, path)
	}

	block := found[0]
	if block.Parent != "" && block.Parent != nginx.Http {
		return serverstate.Block{}, cosy.WrapErrorWithParams(ErrUnsupportedContext, name, block.Parent)
	}
	if len(block.NestedBlocks) > 0 {
		return serverstate.Block{}, cosy.WrapErrorWithParams(ErrNestedBlock, name, block.NestedBlocks[0])
	}
	return block, nil
}

// definedElsewhere returns another configuration file that defines an http
// upstream called name. The group file would duplicate it, which nginx
// rejects. Stream upstreams live in their own namespace and do not clash.
func definedElsewhere(name, sitePath string) string {
	for _, group := range serverstate.List() {
		if group.Name != name || group.Source.Type == serverstate.SourceStream {
			continue
		}
		if samePath(group.ConfigPath, sitePath) {
			continue
		}
		return group.ConfigPath
	}
	return ""
}

func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := nginx.EvalSymlinks(a)
	rb, errB := nginx.EvalSymlinks(b)
	return errA == nil && errB == nil && filepath.Clean(ra) == filepath.Clean(rb)
}

// plainValue reports whether value can be written without quotes or escapes.
func plainValue(value string) bool {
	return value != "" && !strings.ContainsAny(value, " \t\r\n;{}#\"'\\")
}

// toUpstream maps the block onto the managed model. Directives without a form
// field are kept as additional directives, written as they were (quotes
// included), and the comments of the block are carried over as comment lines,
// so the group holds everything the block did.
func toUpstream(name string, block serverstate.Block) (*managed.Upstream, error) {
	directives := make([]managed.Directive, 0, len(block.Directives))
	for _, d := range block.Directives {
		args := make([]string, 0, len(d.Args))
		for i, value := range d.Args {
			raw := d.RawArgs[i]
			if strings.ContainsAny(raw, "\r\n") {
				return nil, cosy.WrapErrorWithParams(ErrMultilineValue, d.Name, name)
			}
			// A quoted value that needs no quotes is the same value to nginx;
			// keep the raw form only when the quotes or escapes matter.
			if plainValue(value) {
				args = append(args, value)
			} else {
				args = append(args, raw)
			}
		}
		directives = append(directives, managed.Directive{Name: d.Name, Params: strings.Join(args, " ")})
	}

	u := managed.FromDirectives(name, directives)

	if len(block.Comments) > 0 {
		lines := make([]string, 0, len(block.Comments)+1)
		if u.ExtraDirectives != "" {
			lines = append(lines, u.ExtraDirectives)
		}
		for _, comment := range block.Comments {
			if strings.ContainsAny(comment, "{}") {
				return nil, cosy.WrapErrorWithParams(ErrCommentWithBraces, name)
			}
			lines = append(lines, strings.TrimSpace(comment))
		}
		u.ExtraDirectives = strings.Join(lines, "\n")
	}
	return u, nil
}

// RemoveBlock cuts content[start:end] out of content. When the block fills its
// lines on its own, the lines go too, and a blank line left doubled by the
// removal is collapsed, so the rest of the file reads as before.
func RemoveBlock(content string, start, end int) string {
	lineStart := start
	for lineStart > 0 && (content[lineStart-1] == ' ' || content[lineStart-1] == '\t') {
		lineStart--
	}
	after := end
	for after < len(content) && (content[after] == ' ' || content[after] == '\t') {
		after++
	}

	wholeLines := lineStart == 0 || content[lineStart-1] == '\n'
	if wholeLines {
		switch {
		case after == len(content):
			end = after
		case content[after] == '\n':
			end = after + 1
		case content[after] == '\r' && after+1 < len(content) && content[after+1] == '\n':
			end = after + 2
		default:
			// Something (a comment) follows the closing brace on its line.
			wholeLines = false
		}
	}
	if !wholeLines {
		return content[:start] + content[end:]
	}

	head, tail := content[:lineStart], content[end:]
	if blankBefore(head) {
		if nl := strings.IndexByte(tail, '\n'); nl >= 0 && strings.TrimSpace(tail[:nl]) == "" {
			tail = tail[nl+1:]
		} else if nl < 0 && strings.TrimSpace(tail) == "" {
			tail = ""
		}
		// At the end of the file, drop the blank line the block followed.
		if tail == "" && head != "" {
			head = strings.TrimSuffix(head, "\n")
			head = strings.TrimSuffix(head, "\r")
		}
	}
	return head + tail
}

// blankBefore reports whether head is empty or ends with a blank line.
func blankBefore(head string) bool {
	if head == "" {
		return true
	}
	trimmed := strings.TrimSuffix(strings.TrimSuffix(head, "\n"), "\r")
	prev := trimmed[strings.LastIndexByte(trimmed, '\n')+1:]
	return strings.TrimSpace(prev) == ""
}

// apply writes both files and runs one `nginx -t` and reload. Any failure puts
// both files back as they were.
func apply(p *plan) error {
	if err := nginx.MkdirAll(managed.Dir(), 0o755); err != nil {
		return cosy.WrapErrorWithParams(managed.ErrConfDirUnavailable, err.Error())
	}
	if err := config.CheckAndCreateHistory(p.sitePath, p.siteContent); err != nil {
		return err
	}

	tx := &config.FileTransaction{}
	if err := tx.Write(p.managedPath, []byte(p.managedContent), 0o644); err != nil {
		return config.RollbackError(err, tx.Rollback)
	}
	if err := tx.Write(p.sitePath, []byte(p.siteContent), 0o644); err != nil {
		return config.RollbackError(err, tx.Rollback)
	}
	return tx.TestAndReload()
}

// managedConfigRecord returns the config record of the new group file, creating
// it like config.Save does, so the file gets its history and sync targets.
func managedConfigRecord(path string) *model.Config {
	q := query.Config
	cfg, err := q.Assign(field.Attrs(&model.Config{
		Filepath: path,
		Name:     filepath.Base(path),
	})).Where(q.Filepath.Eq(path)).FirstOrCreate()
	if err != nil {
		logger.Error(err)
		return &model.Config{Filepath: path, Name: filepath.Base(path)}
	}
	return cfg
}

// rescan refreshes the availability service right away instead of waiting for
// the file watcher. The group file goes first, so the site's proxy_pass is
// already known to name an upstream when the site is scanned.
func rescan(p *plan) {
	if err := upstream.ScanConfig(p.managedPath, []byte(p.managedContent)); err != nil {
		logger.Error(err)
	}
	paths := []string{p.sitePath}
	if enabled, err := site.ResolveEnabledPath(p.siteName); err == nil {
		if ok, err := nginx.Exists(enabled); err == nil && ok {
			paths = append(paths, enabled)
		}
	}
	for _, path := range paths {
		if err := upstream.ScanConfig(path, []byte(p.siteContent)); err != nil {
			logger.Error(err)
		}
	}
}
