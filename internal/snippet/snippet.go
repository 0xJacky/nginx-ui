// Package snippet manages the reusable pieces of nginx configuration kept in
// the snippets directory. A snippet is an ordinary file that sites include or
// copy from the config template panel. Its optional header names and
// describes it, written as comments so nginx can always include the file.
package snippet

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/BurntSushi/toml"
	"github.com/uozi-tech/cosy"
)

const (
	// DirName is the directory below the nginx configuration directory that
	// holds the snippets, where many distributions already keep theirs.
	DirName = "snippets"
	// ReservedDirName below DirName belongs to software that manages its own
	// snippets. It is neither listed nor replicated.
	ReservedDirName = "plugins"
	// maxSnippetBytes bounds one snippet.
	maxSnippetBytes = 256 << 10
)

var (
	fileNamePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.conf$`)
	includePattern     = regexp.MustCompile(`(?m)^[ \t]*include[ \t]+(?:"([^"]+)"|'([^']+)'|([^\s;]+))[ \t]*;`)
	commentPrefixTrims = []string{"# ", "#"}
	errHeaderUnclosed  = errors.New("the header has no end line")
)

// Snippet is one snippet with the fields of its header.
type Snippet struct {
	// File is the file name in the snippets directory.
	File        string                       `json:"file"`
	Name        string                       `json:"name"`
	Description map[string]string            `json:"description"`
	Author      string                       `json:"author"`
	Variables   map[string]template.Variable `json:"variables"`
	// Include is the directive that includes the snippet.
	Include    string    `json:"include"`
	ModifiedAt time.Time `json:"modified_at"`
	// UsedBy lists the configuration files that include the snippet,
	// relative to the nginx configuration directory.
	UsedBy []string `json:"used_by"`
	// Content is the body without the header, set by Get only.
	Content string `json:"content,omitempty"`
}

// header is what the header of a snippet holds.
type header struct {
	Name        string                       `toml:"name,omitempty"`
	Author      string                       `toml:"author,omitempty"`
	Description map[string]string            `toml:"description,omitempty"`
	Variables   map[string]template.Variable `toml:"variables,omitempty"`
}

// Dir is the absolute snippets directory.
func Dir() string {
	return nginx.GetConfPath(DirName)
}

// IncludePath is the path an include directive names. It is relative, so
// nginx resolves it against its configuration directory and the sandbox
// used by nginx -t against its own copy.
func IncludePath(file string) string {
	return DirName + "/" + file
}

// IncludeLine is the directive that includes a path.
func IncludeLine(includePath string) string {
	return "include " + includePath + ";"
}

// ValidFile reports whether file is a valid snippet file name: a .conf
// file of letters, digits, dots, dashes and underscores.
func ValidFile(file string) bool {
	return fileNamePattern.MatchString(file)
}

// List returns the snippets sorted by file name. A snippet whose header does
// not parse is listed under its file name.
func List() ([]Snippet, error) {
	return readAll(usageIndex())
}

func readAll(usage map[string][]string) ([]Snippet, error) {
	entries, err := nginx.ReadDir(Dir())
	if errors.Is(err, fs.ErrNotExist) {
		return []Snippet{}, nil
	}
	if err != nil {
		return nil, err
	}
	snippets := []Snippet{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !ValidFile(entry.Name()) {
			continue
		}
		file := entry.Name()
		s := Snippet{File: file, Include: IncludeLine(IncludePath(file)), UsedBy: usage[IncludePath(file)]}
		if info, err := entry.Info(); err == nil {
			s.ModifiedAt = info.ModTime()
		}
		if h, ok := indexedHeader(file); ok {
			s.applyHeader(h)
		} else if content, err := nginx.ReadFile(filepath.Join(Dir(), file)); err == nil {
			if h, _, err := parse(content); err == nil {
				s.applyHeader(h)
			}
		}
		s.fillDefaults()
		snippets = append(snippets, s)
	}
	slices.SortFunc(snippets, func(a, b Snippet) int { return strings.Compare(a.File, b.File) })
	return snippets, nil
}

func (s *Snippet) applyHeader(h header) {
	s.Name = h.Name
	s.Author = h.Author
	s.Description = h.Description
	s.Variables = h.Variables
}

func (s *Snippet) fillDefaults() {
	if s.Name == "" {
		s.Name = strings.TrimSuffix(s.File, ".conf")
	}
	if s.Description == nil {
		s.Description = map[string]string{}
	}
	if s.Variables == nil {
		s.Variables = map[string]template.Variable{}
	}
	if s.UsedBy == nil {
		s.UsedBy = []string{}
	}
}

// Get returns one snippet with its body.
func Get(file string) (Snippet, error) {
	if !ValidFile(file) {
		return Snippet{}, ErrInvalidFile
	}
	include := IncludePath(file)
	abs := filepath.Join(Dir(), file)
	info, err := nginx.Lstat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return Snippet{}, ErrNotFound
	}
	content, err := nginx.ReadFile(abs)
	if err != nil {
		return Snippet{}, err
	}
	h, body, err := parse(content)
	if err != nil {
		return Snippet{}, cosy.WrapErrorWithParams(ErrInvalidHeader, err.Error())
	}
	s := Snippet{
		File: file, Include: IncludeLine(include),
		ModifiedAt: info.ModTime(), UsedBy: usageIndex()[include], Content: body,
	}
	s.applyHeader(h)
	s.fillDefaults()
	return s, nil
}

// SaveParams is what the user edits of a snippet.
type SaveParams struct {
	File        string
	Name        string
	Description map[string]string
	Content     string
	// Create refuses to replace an existing file.
	Create bool
}

// Save writes a snippet, keeping the author and the variables of its
// header. The configuration is tested and nginx reloaded, and the file is
// replicated to the nodes the snippets directory is deployed to.
func Save(ctx context.Context, p SaveParams, userName string) (Snippet, error) {
	if !ValidFile(p.File) {
		return Snippet{}, ErrInvalidFile
	}
	abs := filepath.Join(Dir(), p.File)

	var h header
	exists, err := nginx.Exists(abs)
	if err != nil {
		return Snippet{}, err
	}
	if exists {
		if p.Create {
			return Snippet{}, ErrAlreadyExists
		}
		if current, err := nginx.ReadFile(abs); err == nil {
			if parsed, _, err := parse(current); err == nil {
				h = parsed
			}
		}
	}
	h.Name = strings.TrimSpace(p.Name)
	h.Description = cleanDescription(p.Description)

	content, err := compose(h, p.Content)
	if err != nil {
		return Snippet{}, err
	}
	if len(content) > maxSnippetBytes {
		return Snippet{}, ErrTooLarge
	}
	if err = nginx.MkdirAll(Dir(), 0o755); err != nil {
		return Snippet{}, err
	}
	if err = config.Save(ctx, abs, string(content), nil, userName); err != nil {
		return Snippet{}, err
	}
	return Get(p.File)
}

func cleanDescription(description map[string]string) map[string]string {
	cleaned := map[string]string{}
	for lang, text := range description {
		if text = strings.TrimSpace(text); text != "" {
			cleaned[lang] = text
		}
	}
	return cleaned
}

// Remover removes a file, tests the configuration and reloads nginx,
// putting the file back when nginx rejects the configuration without it.
type Remover interface {
	Remove(path string) (removed bool, err error)
}

// Delete removes a snippet. A snippet that is still included stays: it is
// reported with the files that include it. It returns the removed path.
func Delete(file string, remover Remover) (string, error) {
	if !ValidFile(file) {
		return "", ErrInvalidFile
	}
	// The scanner may lag behind a change on a remote host, so deleting
	// checks the files as they are now.
	if users := scanUsage()[IncludePath(file)]; len(users) > 0 {
		return "", cosy.WrapErrorWithParams(ErrInUse, strings.Join(users, ", "))
	}
	abs := filepath.Join(Dir(), file)
	removed, err := remover.Remove(abs)
	if err != nil {
		return "", err
	}
	if !removed {
		return "", ErrNotFound
	}
	return abs, nil
}

// Render renders a snippet like a block template, for the config template
// panel of the site editor.
func Render(file string, bindData map[string]template.Variable) (template.ConfigDetail, error) {
	if !ValidFile(file) {
		return template.ConfigDetail{}, ErrInvalidFile
	}
	content, err := nginx.ReadFile(filepath.Join(Dir(), file))
	if err != nil {
		return template.ConfigDetail{}, ErrNotFound
	}
	_, body, err := parse(content)
	if err != nil {
		return template.ConfigDetail{}, cosy.WrapErrorWithParams(ErrInvalidHeader, err.Error())
	}
	return template.RenderBlock(file, []byte(body), bindData)
}

// TemplateInfo describes the snippets like block templates.
func TemplateInfo() []template.ConfigInfoItem {
	snippets, err := readAll(map[string][]string{})
	if err != nil {
		return nil
	}
	items := make([]template.ConfigInfoItem, 0, len(snippets))
	for _, s := range snippets {
		items = append(items, template.ConfigInfoItem{
			Name:        s.Name,
			Description: s.Description,
			Author:      s.Author,
			Filename:    s.File,
			Variables:   s.Variables,
			Origin:      template.OriginCustom,
		})
	}
	return items
}

// parse splits a snippet into its header and body. The header lines are
// comments, so the file stays valid nginx configuration. A file without a
// header is all body.
func parse(content []byte) (h header, body string, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), maxSnippetBytes+1)
	if !scanner.Scan() {
		return h, "", scanner.Err()
	}
	if strings.TrimSpace(scanner.Text()) != template.HeaderStart {
		return h, string(content), nil
	}

	var headerBuf, bodyBuf strings.Builder
	inHeader := true
	for scanner.Scan() {
		line := scanner.Text()
		if inHeader {
			if strings.TrimSpace(line) == template.HeaderEnd {
				inHeader = false
				continue
			}
			line = strings.TrimSpace(line)
			for _, prefix := range commentPrefixTrims {
				if trimmed, ok := strings.CutPrefix(line, prefix); ok {
					line = trimmed
					break
				}
			}
			headerBuf.WriteString(line)
			headerBuf.WriteString("\n")
			continue
		}
		bodyBuf.WriteString(line)
		bodyBuf.WriteString("\n")
	}
	if err = scanner.Err(); err != nil {
		return h, "", err
	}
	if inHeader {
		return h, "", errHeaderUnclosed
	}
	if _, err = toml.Decode(headerBuf.String(), &h); err != nil {
		return h, "", err
	}
	return h, strings.TrimLeft(bodyBuf.String(), "\n"), nil
}

// compose writes the header as comments followed by the body. A snippet
// without a name, description, author or variables has no header.
func compose(h header, body string) ([]byte, error) {
	body = strings.TrimRight(body, "\n") + "\n"
	if h.Name == "" && len(h.Description) == 0 && h.Author == "" && len(h.Variables) == 0 {
		return []byte(body), nil
	}
	var encoded bytes.Buffer
	encoder := toml.NewEncoder(&encoded)
	encoder.Indent = ""
	if err := encoder.Encode(h); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(template.HeaderStart + "\n")
	for _, line := range strings.Split(strings.TrimRight(encoded.String(), "\n"), "\n") {
		if line == "" {
			out.WriteString("#\n")
			continue
		}
		out.WriteString("# " + line + "\n")
	}
	out.WriteString(template.HeaderEnd + "\n\n")
	out.WriteString(body)
	return out.Bytes(), nil
}

// IncludedPaths returns the snippet include paths a configuration names,
// relative to the configuration directory, such as "snippets/gzip.conf". A
// wildcard include names every snippet it matches.
func IncludedPaths(content string) []string {
	confDir := filepath.ToSlash(nginx.GetConfPath())
	var paths []string
	for _, match := range includePattern.FindAllStringSubmatch(stripComments(content), -1) {
		target := match[1] + match[2] + match[3]
		target = strings.TrimPrefix(filepath.ToSlash(target), confDir+"/")
		target = path.Clean(target)
		if !strings.HasPrefix(target, DirName+"/") {
			continue
		}
		paths = append(paths, target)
	}
	return paths
}

// IncludedFiles returns the snippets a configuration includes, as file names
// in the snippets directory.
func IncludedFiles(content string) []string {
	var files []string
	for _, included := range IncludedPaths(content) {
		pattern := strings.TrimPrefix(included, DirName+"/")
		if strings.Contains(pattern, "/") {
			continue
		}
		if !strings.ContainsAny(pattern, "*?[") {
			if ValidFile(pattern) && !slices.Contains(files, pattern) {
				files = append(files, pattern)
			}
			continue
		}
		entries, err := nginx.ReadDir(Dir())
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if ok, _ := path.Match(pattern, entry.Name()); ok && entry.Type().IsRegular() &&
				ValidFile(entry.Name()) && !slices.Contains(files, entry.Name()) {
				files = append(files, entry.Name())
			}
		}
	}
	slices.Sort(files)
	return files
}

// ReadFiles returns the content of snippets by file name. A file that
// cannot be read is left out.
func ReadFiles(files []string) map[string]string {
	contents := map[string]string{}
	for _, file := range files {
		if !ValidFile(file) {
			continue
		}
		if content, err := nginx.ReadFile(filepath.Join(Dir(), file)); err == nil {
			contents[file] = string(content)
		}
	}
	return contents
}

func stripComments(content string) string {
	var out strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

// usageIndex maps every snippet include path to the configuration files
// that include it, relative to the configuration directory. It uses what the
// config scanner recorded, and reads the files itself until the scanner is
// ready.
func usageIndex() map[string][]string {
	if index, ok := indexedUsage(); ok {
		return index
	}
	return scanUsage()
}

// scanUsage builds the usage index by reading nginx.conf, conf.d and the
// sites and streams now.
func scanUsage() map[string][]string {
	confDir := nginx.GetConfPath()
	index := map[string][]string{}
	add := func(rel string) {
		content, err := nginx.ReadFile(filepath.Join(confDir, filepath.FromSlash(rel)))
		if err != nil {
			return
		}
		for _, included := range IncludedPaths(string(content)) {
			if strings.ContainsAny(included, "*?[") {
				for _, target := range expandWildcard(included) {
					index[target] = appendUnique(index[target], rel)
				}
				continue
			}
			index[included] = appendUnique(index[included], rel)
		}
	}

	add("nginx.conf")
	for _, dir := range []string{"conf.d", "sites-available", "streams-available"} {
		entries, err := nginx.ReadDir(filepath.Join(confDir, dir))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				add(dir + "/" + entry.Name())
			}
		}
	}
	for _, users := range index {
		slices.Sort(users)
	}
	return index
}

// expandWildcard lists the snippet include paths a wildcard include matches.
func expandWildcard(pattern string) []string {
	dir := path.Dir(pattern)
	entries, err := nginx.ReadDir(filepath.Join(nginx.GetConfPath(), filepath.FromSlash(dir)))
	if err != nil {
		return nil
	}
	var matches []string
	for _, entry := range entries {
		candidate := dir + "/" + entry.Name()
		if ok, _ := path.Match(pattern, candidate); ok && entry.Type().IsRegular() {
			matches = append(matches, candidate)
		}
	}
	return matches
}

func appendUnique(list []string, value string) []string {
	if slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// NginxRemover removes a file from the configuration of the running nginx: it
// tests the configuration without the file and reloads, and puts the file
// back when either step fails.
type NginxRemover struct{}

// Remove implements Remover.
func (NginxRemover) Remove(path string) (bool, error) {
	release := config.LockApply()
	defer release()

	exists, err := nginx.Exists(path)
	if err != nil || !exists {
		return false, err
	}
	snapshot, err := config.CaptureFile(path)
	if err != nil {
		return false, err
	}
	if err = nginx.Remove(path); err != nil {
		return false, err
	}
	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return false, config.RollbackError(result.GetError(), func() error { return snapshot.Restore(path) })
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		return false, config.RollbackError(result.GetError(), func() error { return config.RestoreAndReload(path, snapshot) })
	}
	return true, nil
}
