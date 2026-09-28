package clustersync

import (
	"context"
	"fmt"
	"path"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/access_list"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// accessListItem replicates the rendered access lists a set of sites and
// streams include. It is blocking: a node without the list files would reject
// every site that includes them in its Nginx test.
func accessListItem(slugs []string) (item, bool) {
	files := accessListConfigFiles(slugs)
	if len(files) == 0 {
		return item{}, false
	}
	return newConfigBatchItem(fmt.Sprintf("access lists (%d)", len(files)), files, true, true), true
}

func accessListConfigFiles(slugs []string) []ConfigFile {
	rendered := access_list.ReadFiles(slugs)
	files := make([]ConfigFile, 0, len(rendered))
	for rel, content := range rendered {
		files = append(files, ConfigFile{
			BaseDir: path.Dir(rel),
			Name:    path.Base(rel),
			Content: content,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files
}

// includedAccessLists returns the access lists the given files include.
func includedAccessLists(files []ConfigFile) []string {
	seen := map[string]bool{}
	var slugs []string
	for _, file := range files {
		found, err := access_list.Slugs(file.Content)
		if err != nil {
			continue
		}
		for _, slug := range found {
			if !seen[slug] {
				seen[slug] = true
				slugs = append(slugs, slug)
			}
		}
	}
	sort.Strings(slugs)
	return slugs
}

// PushAccessLists replicates the rendered files of the given access lists to
// the nodes. The receiving nodes reload Nginx once after writing them.
func PushAccessLists(ctx context.Context, slugs []string, nodeIDs []uint64) (*Summary, error) {
	nodes, err := resolveNodes(nodeIDs)
	if err != nil {
		return nil, err
	}
	pushItem, ok := accessListItem(slugs)
	if len(nodes) == 0 || !ok {
		return (&collector{}).summary(), nil
	}
	return run(ctx, nodes, []item{pushItem}), nil
}

// PushAccessListsForContent replicates the access lists a site or stream file
// includes before the file itself is pushed to the nodes.
func PushAccessListsForContent(ctx context.Context, content string, nodeIDs []uint64) error {
	slugs, err := access_list.Slugs(content)
	if err != nil || len(slugs) == 0 {
		return nil
	}
	summary, err := PushAccessLists(ctx, slugs, nodeIDs)
	if err != nil {
		return err
	}
	if summary.Failed > 0 {
		return fmt.Errorf("pushing access lists failed on %d node(s)", summary.Failed)
	}
	return nil
}

// PushAccessListsToNodes is PushAccessListsForContent for callers that hold
// node records. Failures are logged: the site push that follows reports the
// resulting Nginx test error to the user.
func PushAccessListsToNodes(content string, nodes []*model.Node) {
	if len(nodes) == 0 {
		return
	}
	ids := make([]uint64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	if err := PushAccessListsForContent(context.Background(), content, ids); err != nil {
		logger.Errorf("Pushing access lists to remote nodes failed: %v", err)
	}
}
