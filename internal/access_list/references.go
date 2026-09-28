package access_list

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy/logger"
)

// Reference kinds.
const (
	RefKindSite   = "site"
	RefKindStream = "stream"
)

// Reference is one include of an access list in a site or stream file.
type Reference struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Server     int    `json:"server"`
	ServerName string `json:"server_name"`
	// Location is empty for a server level include.
	Location string `json:"location,omitempty"`
	Slug     string `json:"slug"`
}

// referenceDirs maps the kinds of files that can include an access list to
// the directories that hold them.
var referenceDirs = map[string]string{
	RefKindSite:   "sites-available",
	RefKindStream: "streams-available",
}

// ScanReferences reads every site and stream file and returns the access list
// includes they contain. Unparsable files are skipped: they cannot include a
// list in a way Nginx would load either.
func ScanReferences() []Reference {
	var refs []Reference
	kinds := make([]string, 0, len(referenceDirs))
	for kind := range referenceDirs {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	for _, kind := range kinds {
		dir := nginx.GetConfPath(referenceDirs[kind])
		entries, err := nginx.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.Warnf("Scanning %s for access list references failed: %v", dir, err)
			}
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			content, err := nginx.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			found, err := ReferencesIn(string(content))
			if err != nil {
				continue
			}
			for _, ref := range found {
				ref.Kind = kind
				ref.Name = entry.Name()
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

// ReferencesIn returns every access list include of a configuration file,
// including includes inside blocks Nginx UI does not manage.
func ReferencesIn(content string) ([]Reference, error) {
	roots, err := parseStatements(content)
	if err != nil {
		return nil, err
	}

	var refs []Reference
	for i, server := range serverBlocks(roots) {
		name := serverName(server.Children)
		walkIncludes(server, func(slug string, location *statement) {
			ref := Reference{Server: i, ServerName: name, Slug: slug}
			if location != nil {
				ref.Location = locationPath(location)
			}
			refs = append(refs, ref)
		})
	}
	return refs, nil
}

// walkIncludes calls fn for every access list include below block, together
// with the innermost location that contains it.
func walkIncludes(block *statement, fn func(slug string, location *statement)) {
	var walk func(children []*statement, location *statement)
	walk = func(children []*statement, location *statement) {
		for _, st := range children {
			if st.Name == "include" && len(st.Args) == 1 {
				if slug, ok := SlugFromInclude(st.Args[0]); ok {
					fn(slug, location)
				}
			}
			if st.hasBody() {
				inner := location
				if st.Name == "location" {
					inner = st
				}
				walk(st.Children, inner)
			}
		}
	}
	walk(block.Children, nil)
}

// Slugs returns every access list a configuration file includes.
func Slugs(content string) ([]string, error) {
	refs, err := ReferencesIn(content)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	slugs := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !seen[ref.Slug] {
			seen[ref.Slug] = true
			slugs = append(slugs, ref.Slug)
		}
	}
	return slugs, nil
}

// FilterReferences keeps the references to the given slugs.
func FilterReferences(refs []Reference, slugs ...string) []Reference {
	want := map[string]bool{}
	for _, slug := range slugs {
		want[slug] = true
	}
	var result []Reference
	for _, ref := range refs {
		if want[ref.Slug] {
			result = append(result, ref)
		}
	}
	return result
}

// describeReferences renders references for an error message.
func describeReferences(refs []Reference, lists []string) string {
	parts := make([]string, 0, len(refs)+len(lists))
	for _, name := range lists {
		parts = append(parts, name)
	}
	for _, ref := range refs {
		part := ref.Kind + " " + ref.Name
		if ref.Location != "" {
			part += " (location " + ref.Location + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}
