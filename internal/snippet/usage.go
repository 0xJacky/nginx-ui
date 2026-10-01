package snippet

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// usage keeps the snippet includes of every configuration file, fed by the
// config scanner: once when it starts and again for every file that changes.
// Listing snippets then reads no file at all, which matters most when the
// configuration lives on a remote host.
var usage = struct {
	sync.RWMutex
	// includes maps a file, relative to the configuration directory, to the
	// snippet include paths it names.
	includes map[string][]string
	// headers holds the parsed header of every snippet, by file name.
	headers map[string]header
	// ready is set once the scanner went through every file.
	ready bool
}{includes: map[string][]string{}, headers: map[string]header{}}

func init() {
	cache.RegisterCallback("snippet.scanIncludes", scanIncludes)
	cache.RegisterPostScanCallback(func() {
		usage.Lock()
		usage.ready = true
		usage.Unlock()
	})
}

// scanIncludes records the snippet includes of one file. Empty content is a
// removed file.
func scanIncludes(configPath string, content []byte) error {
	rel, err := filepath.Rel(nginx.GetConfPath(), configPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	rel = filepath.ToSlash(rel)
	included := IncludedPaths(string(content))

	usage.Lock()
	defer usage.Unlock()
	if len(included) == 0 {
		delete(usage.includes, rel)
	} else {
		usage.includes[rel] = included
	}

	if file, ok := strings.CutPrefix(rel, DirName+"/"); ok && ValidFile(file) {
		h, _, err := parse(content)
		if len(content) == 0 || err != nil {
			delete(usage.headers, file)
		} else {
			usage.headers[file] = h
		}
	}
	return nil
}

// indexedHeader returns the header the scanner recorded for a snippet.
func indexedHeader(file string) (header, bool) {
	usage.RLock()
	defer usage.RUnlock()
	if !usage.ready {
		return header{}, false
	}
	h, ok := usage.headers[file]
	return h, ok
}

// indexedUsage builds the usage index from what the scanner recorded. ok is
// false until the scanner finished its first pass.
func indexedUsage() (index map[string][]string, ok bool) {
	usage.RLock()
	if !usage.ready {
		usage.RUnlock()
		return nil, false
	}
	includes := make(map[string][]string, len(usage.includes))
	for file, paths := range usage.includes {
		includes[file] = paths
	}
	usage.RUnlock()

	index = map[string][]string{}
	expanded := map[string][]string{}
	for file, paths := range includes {
		for _, included := range paths {
			targets := []string{included}
			if strings.ContainsAny(included, "*?[") {
				if _, seen := expanded[included]; !seen {
					expanded[included] = expandWildcard(included)
				}
				targets = expanded[included]
			}
			for _, target := range targets {
				index[target] = appendUnique(index[target], file)
			}
		}
	}
	for _, users := range index {
		slices.Sort(users)
	}
	return index, true
}
