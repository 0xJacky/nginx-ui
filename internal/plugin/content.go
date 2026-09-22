package plugin

// This file covers the content block of a manifest (spec 17): the templates
// and translation files a plugin ships as data, which the host reads while
// the plugin is enabled and checks before a package is installed.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/pofile"
	"github.com/uozi-tech/cosy"
)

// maxLocaleFileSize bounds one translation file.
const maxLocaleFileSize = 4 << 20

// localeFileSuffix ends the name of every translation file.
const localeFileSuffix = ".po"

// ContentEntry is what one enabled plugin contributes through its content
// block. A directory is empty when the plugin does not declare it or it
// does not resolve inside the plugin directory.
type ContentEntry struct {
	PluginID string
	// TemplatesDir is the absolute directory holding conf/ and block/.
	TemplatesDir string
	// LocalesDir is the absolute directory holding the <lang>.po files.
	LocalesDir string
}

// ContentEntries lists the content of every enabled plugin whose approval
// matches its manifest, ordered by plugin id.
func (m *Manager) ContentEntries() []ContentEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var entries []ContentEntry
	for _, item := range m.entries {
		if !runnableLocked(item) || item.manifest.Content == nil {
			continue
		}
		content := item.manifest.Content
		entry := ContentEntry{
			PluginID:     item.id,
			TemplatesDir: contentDir(item.dir, content.Templates),
			LocalesDir:   contentDir(item.dir, content.Locales),
		}
		if entry.TemplatesDir == "" && entry.LocalesDir == "" {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].PluginID < entries[j].PluginID })
	return entries
}

// contentDir resolves a manifest relative directory. A path that is not
// safe, is missing or leads outside the plugin directory through a symlink
// yields an empty string.
func contentDir(pluginDir, rel string) string {
	if rel == "" || !isSafeRelPath(rel) {
		return ""
	}
	root, err := filepath.EvalSymlinks(pluginDir)
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(pluginDir, filepath.FromSlash(rel)))
	if err != nil || !isInside(root, resolved) {
		return ""
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return ""
	}
	return resolved
}

// ContentProblem is one issue CheckContent found.
type ContentProblem struct {
	Level Level
	// Rule is the spec requirement, e.g. "CONTENT-3".
	Rule string
	// Path is the package relative path the problem is about.
	Path    string
	Message string
}

func (p ContentProblem) String() string {
	if p.Path == "" {
		return p.Message
	}
	return p.Path + ": " + p.Message
}

// CheckContent checks the files the content block of a manifest points at,
// in a plugin directory (spec CONTENT-2, CONTENT-3, CONTENT-6, CONTENT-7).
// Unsafe paths are MAN-18 and left to the manifest checks.
func CheckContent(manifest *protocol.Manifest, dir string) []ContentProblem {
	if manifest == nil || manifest.Content == nil {
		return nil
	}
	var problems []ContentProblem
	if rel := manifest.Content.Templates; rel != "" && isSafeRelPath(rel) {
		problems = append(problems, checkTemplates(dir, rel)...)
	}
	if rel := manifest.Content.Locales; rel != "" && isSafeRelPath(rel) {
		problems = append(problems, checkLocales(dir, rel)...)
	}
	return problems
}

// validateContentFiles rejects a package whose content does not parse, as
// install does (spec CONTENT-4, CONTENT-7).
func validateContentFiles(manifest *protocol.Manifest, dir string) error {
	for _, problem := range CheckContent(manifest, dir) {
		if problem.Level == LevelError {
			return cosy.WrapErrorWithParams(ErrContentInvalid, problem.Path, problem.Message)
		}
	}
	return nil
}

// contentRoot checks that a declared content directory exists and is a
// directory.
func contentRoot(dir, rel, rule, field string) (string, []ContentProblem) {
	root := filepath.Join(dir, filepath.FromSlash(rel))
	info, err := os.Lstat(root)
	switch {
	case err != nil:
		return "", []ContentProblem{{Level: LevelError, Rule: rule, Path: rel,
			Message: fmt.Sprintf("content.%s points at a missing directory", field)}}
	case !info.IsDir():
		return "", []ContentProblem{{Level: LevelError, Rule: rule, Path: rel,
			Message: fmt.Sprintf("content.%s must be a directory", field)}}
	}
	return root, nil
}

func checkTemplates(dir, rel string) []ContentProblem {
	root, problems := contentRoot(dir, rel, "CONTENT-2", "templates")
	if root == "" {
		return problems
	}
	fsys := os.DirFS(root)

	entries, err := os.ReadDir(root)
	if err != nil {
		return append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-2", Path: rel, Message: err.Error()})
	}
	for _, entry := range entries {
		if entry.IsDir() && slices.Contains(template.Kinds, entry.Name()) {
			continue
		}
		problems = append(problems, ContentProblem{Level: LevelWarning, Rule: "CONTENT-2", Path: path.Join(rel, entry.Name()),
			Message: "only the conf and block directories hold templates, this entry is ignored"})
	}

	count := 0
	for _, kind := range template.Kinds {
		kindEntries, err := os.ReadDir(filepath.Join(root, kind))
		if err != nil {
			continue
		}
		for _, entry := range kindEntries {
			name := entry.Name()
			where := path.Join(rel, kind, name)
			if !entry.Type().IsRegular() || !template.IsValidFileName(name) {
				problems = append(problems, ContentProblem{Level: LevelWarning, Rule: "CONTENT-2", Path: where,
					Message: "not a template (a regular file named like name.conf), ignored"})
				continue
			}
			count++
			if err := template.ValidateFile(fsys, kind, name); err != nil {
				problems = append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-3", Path: where, Message: err.Error()})
				continue
			}
			if !template.HasName(fsys, kind, name) {
				problems = append(problems, ContentProblem{Level: LevelWarning, Rule: "CONTENT-3", Path: where,
					Message: "the template has no name and is listed under its file name"})
			}
		}
	}
	if count == 0 {
		problems = append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-2", Path: rel,
			Message: "content.templates holds no template in conf/ or block/"})
	}
	return problems
}

func checkLocales(dir, rel string) []ContentProblem {
	root, problems := contentRoot(dir, rel, "CONTENT-6", "locales")
	if root == "" {
		return problems
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-6", Path: rel, Message: err.Error()})
	}

	count := 0
	for _, entry := range entries {
		name := entry.Name()
		where := path.Join(rel, name)
		lang, isPO := strings.CutSuffix(name, localeFileSuffix)
		if !isPO || !entry.Type().IsRegular() {
			problems = append(problems, ContentProblem{Level: LevelWarning, Rule: "CONTENT-6", Path: where,
				Message: "not a translation file (<lang>.po), ignored"})
			continue
		}
		count++
		if !translation.IsLanguage(lang) {
			problems = append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-6", Path: where,
				Message: fmt.Sprintf("%q is not a language of the host (%s)", lang, strings.Join(translation.Languages(), ", "))})
			continue
		}
		if _, err := readLocaleFile(filepath.Join(root, name)); err != nil {
			problems = append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-7", Path: where, Message: err.Error()})
		}
	}
	if count == 0 {
		problems = append(problems, ContentProblem{Level: LevelError, Rule: "CONTENT-6", Path: rel,
			Message: "content.locales holds no .po file"})
	}
	return problems
}

// readLocaleFile parses one translation file.
func readLocaleFile(file string) (pofile.Dict, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > maxLocaleFileSize {
		return nil, fmt.Errorf("the file is larger than %d bytes", maxLocaleFileSize)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return translation.ParsePO(data)
}

// LoadLocales reads the translation files of a locales directory, one
// catalog per host language. A file that does not parse is skipped and
// reported in errs.
func LoadLocales(dir string) (catalog translation.Catalog, errs []error) {
	catalog = translation.Catalog{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return catalog, []error{err}
	}
	for _, entry := range entries {
		lang, isPO := strings.CutSuffix(entry.Name(), localeFileSuffix)
		if !isPO || !entry.Type().IsRegular() || !translation.IsLanguage(lang) {
			continue
		}
		dict, err := readLocaleFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		catalog[lang] = dict
	}
	return catalog, errs
}
