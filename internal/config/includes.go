package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

const (
	maxConfigIncludeDepth = 16
	maxConfigIncludeFiles = 1024
	maxConfigIncludeBytes = 64 << 20
)

type configIncludeValidator struct {
	base    string
	active  map[string]bool
	visited map[string]bool
	bytes   int
	files   int
	opts    validateOptions
	// pattern is the argument of the include statement in the edited file
	// that is being expanded, used to name the source of an error.
	pattern string
}

// validateConfigIncludes applies the directive policy to existing include
// targets as well as the edited file. Relative includes use Nginx's main config
// prefix, not the directory of the file containing the include. Globs with no
// matches remain valid, and safe system snippets outside the conf root work.
func validateConfigIncludes(path string, content []byte, opts validateOptions) error {
	validator := configIncludeValidator{
		base:    includeBasePath(),
		active:  make(map[string]bool),
		visited: make(map[string]bool),
		opts:    opts,
	}
	return validator.validate(path, content, 0)
}

// includeBasePath returns the directory relative include patterns resolve
// against: Nginx's main config prefix.
func includeBasePath() string {
	if settings.NginxSettings.ConfigPath != "" && filepath.IsAbs(settings.NginxSettings.ConfigPath) {
		return filepath.Dir(settings.NginxSettings.ConfigPath)
	}
	return nginx.GetConfPath()
}

// canonicalIncludePath returns the path with symlinks resolved when possible,
// so one file reached through different paths is recognized as the same.
func canonicalIncludePath(path string) string {
	if resolved, err := nginx.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func (validator *configIncludeValidator) validate(path string, content []byte, depth int) error {
	if depth > maxConfigIncludeDepth || validator.files >= maxConfigIncludeFiles || validator.bytes+len(content) > maxConfigIncludeBytes {
		return fmt.Errorf("nginx include validation limit exceeded")
	}
	canonical := canonicalIncludePath(path)
	if validator.active[canonical] {
		return fmt.Errorf("cyclic nginx include: %s", path)
	}
	if validator.visited[canonical] {
		return nil
	}
	validator.active[canonical] = true
	defer delete(validator.active, canonical)
	validator.files++
	validator.bytes += len(content)
	if err := validator.validateContent(path, content, depth); err != nil {
		return err
	}
	if err := visitConfigStatementsAsNginx(content, func(statement []string) error {
		if len(statement) != 2 || !strings.EqualFold(statement[0], "include") {
			return nil
		}
		if depth == 0 {
			validator.pattern = statement[1]
		}
		pattern := includeFullPath(validator.base, statement[1])
		matches, err := resolveIncludePattern(pattern)
		if err != nil {
			if _, ok := errors.AsType[*cosy.Error](err); ok {
				return err
			}
			return fmt.Errorf("resolve nginx include %s: %w", pattern, err)
		}
		for _, match := range matches {
			info, err := nginx.Stat(match)
			if err != nil {
				return fmt.Errorf("stat nginx include %s: %w", match, err)
			}
			if info.Size() > int64(maxConfigIncludeBytes-validator.bytes) {
				return fmt.Errorf("nginx include validation limit exceeded")
			}
			included, err := nginx.ReadFile(match)
			if err != nil {
				return fmt.Errorf("read nginx include %s: %w", match, err)
			}
			if err = validator.validate(match, included, depth+1); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	validator.visited[canonical] = true
	return nil
}

// validateContent applies the content policy to one file and names that file
// on every restricted directive it finds, since the statement validation only
// knows the directive itself. The edited file is held to the full policy; a
// file it includes from disk is held to the relaxed one, which accepts the
// load_module statement a distribution's own modules-enabled snippet wrote
// (#2087).
func (validator *configIncludeValidator) validateContent(path string, content []byte, depth int) error {
	opts := validator.opts

	var found []RestrictedDirective
	if validator.opts.found != nil {
		opts.found = &found
	}

	var err error
	if depth == 0 {
		err = validateConfigContent(content, opts)
	} else {
		err = validateIncludedConfigContent(content, opts)
	}

	// The edited file is named by the path the caller asked to save. An
	// included file is named the way includedDirectiveError names it, so the
	// confirmation an administrator sees never reveals a path outside the
	// config directory either.
	name := path
	if depth > 0 {
		name = validator.includedFileName(path)
	}

	if validator.opts.found != nil {
		for index := range found {
			found[index].File = name
		}
		*validator.opts.found = append(*validator.opts.found, found...)
	}

	if restricted, ok := errors.AsType[*RestrictedDirectiveError](err); ok {
		rejection := restricted.err
		if depth > 0 {
			rejection = validator.includedDirectiveError(path, rejection)
		}
		return &RestrictedDirectiveError{
			Found: RestrictedDirective{Directive: restricted.Found.Directive, File: name},
			err:   rejection,
		}
	}

	if err != nil && depth > 0 {
		return validator.includedDirectiveError(path, err)
	}

	return err
}

// includedDirectiveError names the included file that holds a restricted
// directive, since the edited file itself may contain none. A file is named by
// its path only when that path lies inside the Nginx config directory without
// leaving it through "..". Any other file is named by the include argument
// written in the edited file, so the error never reveals a file name outside
// the config directory that the caller did not write.
func (validator *configIncludeValidator) includedDirectiveError(path string, err error) error {
	cErr, ok := errors.AsType[*cosy.Error](err)
	if !ok || len(cErr.Params) == 0 {
		return err
	}
	wantErr, ok := errors.AsType[*cosy.Error](ErrConfigDirectiveNotAllowed)
	if !ok || cErr.Scope != wantErr.Scope || cErr.Code != wantErr.Code {
		return err
	}

	return cosy.WrapErrorWithParams(ErrIncludedDirectiveNotAllowed, validator.includedFileName(path), cErr.Params[0])
}

// includedFileName is the name an included file is reported under: its own
// path when that path lies inside the Nginx config directory, otherwise the
// include argument the edited file wrote. See includedDirectiveError.
func (validator *configIncludeValidator) includedFileName(path string) string {
	if isPlainPathInConfDir(path) {
		return filepath.Clean(path)
	}
	return validator.pattern
}

func isPlainPathInConfDir(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == ".." {
			return false
		}
	}
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(nginx.GetConfPath())+string(filepath.Separator)) {
		return false
	}
	// A symlinked directory inside the conf dir can resolve to a location
	// outside it, so a name under the conf dir may belong to a file that does
	// not. Only name a file whose symlink-resolved path still lies under the
	// symlink-resolved conf dir; anything else falls back to the include
	// argument. Both sides are resolved so a conf dir that is itself reached
	// through a symlink (such as /var on macOS) still matches.
	confPath := canonicalIncludePath(nginx.GetConfPath())
	return strings.HasPrefix(canonicalIncludePath(path), confPath+string(filepath.Separator))
}

type includeWalker struct {
	base   string
	active map[string]bool
	bytes  int
	files  int
	visit  func(statement []string, depth int)
}

// VisitStatementsWithIncludes calls visit for every statement of a
// configuration and, right after each include, for the statements of the files
// it matches, so the statements arrive in the order Nginx reads them. depth is
// 0 for the statements of content itself and grows by one per include level.
// path may be empty when content is not saved yet.
//
// The walk never fails: includes built from variables, globs without matches,
// and files that cannot be read are skipped, and expansion stops at include
// cycles and at the limits the include validation applies.
func VisitStatementsWithIncludes(path string, content []byte, visit func(statement []string, depth int)) {
	walker := includeWalker{base: includeBasePath(), active: make(map[string]bool), visit: visit}
	walker.walk(path, content, 0)
}

func (walker *includeWalker) walk(path string, content []byte, depth int) {
	if path != "" {
		canonical := canonicalIncludePath(path)
		if walker.active[canonical] {
			return
		}
		walker.active[canonical] = true
		defer delete(walker.active, canonical)
	}
	walker.files++
	walker.bytes += len(content)

	_ = visitConfigStatementsAsNginx(content, func(statement []string) error {
		if len(statement) == 0 {
			return nil
		}
		walker.visit(statement, depth)
		if len(statement) == 2 && strings.EqualFold(statement[0], "include") && depth < maxConfigIncludeDepth {
			walker.include(statement[1], depth+1)
		}
		return nil
	})
}

func (walker *includeWalker) include(pattern string, depth int) {
	if strings.Contains(pattern, "$") {
		return
	}
	matches, err := resolveIncludePattern(includeFullPath(walker.base, pattern))
	if err != nil {
		return
	}
	for _, match := range matches {
		if walker.files >= maxConfigIncludeFiles {
			return
		}
		info, err := nginx.Stat(match)
		if err != nil || info.IsDir() || info.Size() > int64(maxConfigIncludeBytes-walker.bytes) {
			continue
		}
		included, err := nginx.ReadFile(match)
		if err != nil || walker.bytes+len(included) > maxConfigIncludeBytes {
			continue
		}
		walker.walk(match, included, depth)
	}
}
