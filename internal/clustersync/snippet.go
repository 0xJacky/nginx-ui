package clustersync

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/snippet"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// snippetItem replicates the snippets a set of sites and streams include. It
// is blocking, since a node without them rejects every site that includes
// one in its Nginx test. It never overwrites: a node may keep its own copy,
// such as the snippets its distribution ships, and the snippets directory
// sync is where a person decides to replace them.
func snippetItem(files []string) (item, bool) {
	contents := snippet.ReadFiles(files)
	if len(contents) == 0 {
		return item{}, false
	}
	configFiles := make([]ConfigFile, 0, len(contents))
	for name, content := range contents {
		configFiles = append(configFiles, ConfigFile{BaseDir: snippet.DirName, Name: name, Content: content})
	}
	sort.Slice(configFiles, func(i, j int) bool { return configFiles[i].Name < configFiles[j].Name })
	return newConfigBatchItem(fmt.Sprintf("snippets (%d)", len(configFiles)), configFiles, false, true), true
}

// includedSnippets returns the snippets the given files include.
func includedSnippets(files []ConfigFile) []string {
	var names []string
	for _, file := range files {
		for _, name := range snippet.IncludedFiles(file.Content) {
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// PushSnippetsToNodes creates the snippets a site or stream file includes on
// the nodes that miss them, before the file itself is pushed. Failures are
// logged: the site push that follows reports the resulting Nginx test error.
func PushSnippetsToNodes(content string, nodes []*model.Node) {
	if len(nodes) == 0 {
		return
	}
	files := snippet.IncludedFiles(content)
	if len(files) == 0 {
		return
	}
	pushItem, ok := snippetItem(files)
	if !ok {
		return
	}
	ids := make([]uint64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	refs, err := resolveNodes(ids)
	if err != nil || len(refs) == 0 {
		return
	}
	summary := run(context.Background(), refs, []item{pushItem})
	if summary.Failed > 0 {
		logger.Errorf("Pushing snippets to remote nodes failed on %d node(s)", summary.Failed)
	}
}
