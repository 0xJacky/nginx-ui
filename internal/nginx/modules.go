package nginx

import (
	"regexp"
	"strings"
)

const (
	ModuleStream = "stream_module"
)

func GetModules() (modules []string) {
	out := getNginxV()
	
	// Regular expression to find modules in nginx -V output
	r := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(_module)?`)
	
	// Find all matches
	matches := r.FindAllStringSubmatch(out, -1)
	
	// Extract module names from matches
	for _, match := range matches {
		module := match[1]
		// If the module doesn't end with "_module", add it
		if !strings.HasSuffix(module, "_module") {
			module = module + "_module"
		}
		modules = append(modules, module)
	}

	return modules
}

func IsModuleLoaded(module string) bool {
	modules := GetModules()
	
	for _, m := range modules {
		if m == module {
			return true
		}
	}
	
	return false
}