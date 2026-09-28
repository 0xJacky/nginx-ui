package serverstate

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
)

// Source types of the file an upstream block is defined in.
const (
	SourceManaged = "managed"
	SourceSite    = "site"
	SourceStream  = "stream"
	SourceConfig  = "config"
)

// Source identifies the file an upstream block lives in. Name is the site or
// stream name, the managed upstream name, or the path relative to the nginx
// configuration directory for any other file.
type Source struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// Server is the state of one `server` line of an upstream block.
type Server struct {
	Address string `json:"address"`
	// Socket is the host:port key of the availability results.
	Socket string `json:"socket"`
	Down   bool   `json:"down"`
	Backup bool   `json:"backup"`
	Weight int    `json:"weight,omitempty"`
	// Params holds the other parameters (max_fails, slow_start, ...).
	Params string `json:"params,omitempty"`
}

// Group is one upstream block together with the file that defines it.
type Group struct {
	Name       string   `json:"name"`
	ConfigPath string   `json:"config_path"`
	Source     Source   `json:"source"`
	Servers    []Server `json:"servers"`
}

// resolvePath follows symlinks so a site reached through sites-enabled and
// sites-available is recognized as one file.
func resolvePath(path string) string {
	if resolved, err := nginx.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b) || resolvePath(a) == resolvePath(b)
}

// managedName reports the managed upstream stored at path, if any. A file that
// only follows the naming scheme but was edited by hand is not managed.
func managedName(path string) (string, bool) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "upstream-") || !strings.HasSuffix(base, ".conf") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(base, "upstream-"), ".conf")
	managedPath, err := managed.Path(name)
	if err != nil || !samePath(managedPath, path) {
		return "", false
	}
	if _, _, _, err := managed.Read(name); err != nil {
		return "", false
	}
	return name, true
}

// classify works out which kind of file path is and the canonical path to
// save it through: sites and streams are always written in their *-available
// directory, like their editors do.
func classify(path string) (Source, string) {
	resolved := resolvePath(path)
	dir := filepath.Dir(resolved)
	base := filepath.Base(resolved)

	if dir == resolvePath(nginx.GetConfPath("sites-available")) {
		return Source{Type: SourceSite, Name: base}, resolved
	}
	if dir == resolvePath(nginx.GetConfPath("streams-available")) {
		return Source{Type: SourceStream, Name: base}, resolved
	}
	if name, ok := managedName(path); ok {
		return Source{Type: SourceManaged, Name: name}, path
	}

	name := path
	if rel, err := filepath.Rel(nginx.GetConfPath(), path); err == nil && !strings.HasPrefix(rel, "..") {
		name = rel
	}
	return Source{Type: SourceConfig, Name: name}, path
}

func toServer(line ServerLine) Server {
	return Server{
		Address: line.Address,
		Socket:  upstream.SocketAddress(line.Address),
		Down:    line.Down(),
		Backup:  line.Backup(),
		Weight:  line.Weight(),
		Params:  line.OtherParams(),
	}
}

func toGroups(content, path string, source Source) []Group {
	blocks := ParseBlocks(content)
	groups := make([]Group, 0, len(blocks))
	for _, block := range blocks {
		servers := make([]Server, 0, len(block.Servers))
		for _, line := range block.Servers {
			servers = append(servers, toServer(line))
		}
		groups = append(groups, Group{
			Name:       block.Name,
			ConfigPath: path,
			Source:     source,
			Servers:    servers,
		})
	}
	return groups
}

// readGroups parses the upstream blocks of one configuration file.
func readGroups(path string) ([]Group, error) {
	source, canonical := classify(path)
	content, err := nginx.ReadFile(canonical)
	if err != nil {
		return nil, err
	}
	return toGroups(string(content), canonical, source), nil
}

// List returns every upstream block of every configuration file the upstream
// scanner knows to define one, sorted by upstream name and file. Each file is
// read once, even when it is reachable through a symlink as well.
func List() []Group {
	seen := make(map[string]bool)
	groups := make([]Group, 0)
	for _, path := range upstream.GetUpstreamService().GetUpstreamConfigPaths() {
		resolved := resolvePath(path)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true

		fileGroups, err := readGroups(path)
		if err != nil {
			continue
		}
		groups = append(groups, fileGroups...)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Name != groups[j].Name {
			return groups[i].Name < groups[j].Name
		}
		return groups[i].ConfigPath < groups[j].ConfigPath
	})
	return groups
}
