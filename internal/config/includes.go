package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
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
}

// validateConfigIncludes applies the directive policy to existing include
// targets as well as the edited file. Relative includes use Nginx's main config
// prefix, not the directory of the file containing the include. Globs with no
// matches remain valid, and safe system snippets outside the conf root work.
func validateConfigIncludes(path string, content []byte) error {
	base := nginx.GetConfPath()
	if settings.NginxSettings.ConfigPath != "" && filepath.IsAbs(settings.NginxSettings.ConfigPath) {
		base = filepath.Dir(settings.NginxSettings.ConfigPath)
	}
	validator := configIncludeValidator{base: base, active: make(map[string]bool), visited: make(map[string]bool)}
	return validator.validate(path, content, 0)
}

func (validator *configIncludeValidator) validate(path string, content []byte, depth int) error {
	if depth > maxConfigIncludeDepth || validator.files >= maxConfigIncludeFiles || validator.bytes+len(content) > maxConfigIncludeBytes {
		return fmt.Errorf("nginx include validation limit exceeded")
	}
	canonical := filepath.Clean(path)
	if resolved, err := nginx.EvalSymlinks(path); err == nil {
		canonical = filepath.Clean(resolved)
	}
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
	if err := ValidateConfigContentBytes(content); err != nil {
		return err
	}
	if err := visitConfigStatements(content, func(statement []string) error {
		if len(statement) != 2 || !strings.EqualFold(statement[0], "include") {
			return nil
		}
		pattern := statement[1]
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(validator.base, pattern)
		}
		matches, err := nginx.Glob(pattern)
		if err != nil {
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
