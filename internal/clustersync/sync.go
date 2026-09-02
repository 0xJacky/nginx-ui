package clustersync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

// SyncDirectory replicates every configuration file below dir to the given
// nodes. It is what turns file-by-file deployment into directory deployment.
func SyncDirectory(ctx context.Context, dir string, nodeIDs []uint64, overwrite bool) (*Summary, error) {
	nodes, err := resolveNodes(nodeIDs)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrNoTargetNode
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrPathOutsideConfDir
	}

	files, err := CollectConfigFiles(dir)
	if err != nil {
		return nil, err
	}

	relative, err := filepath.Rel(filepath.Clean(nginx.GetConfPath()), filepath.Clean(dir))
	if err != nil || relative == "." {
		relative = "/"
	}

	label := fmt.Sprintf("%s (%d)", filepath.ToSlash(relative), len(files))

	return run(ctx, nodes, []item{configBatchItem(label, files, overwrite)}), nil
}

// SyncNodes replicates the selected kinds of local content to the given nodes.
// A full scope is the one-click "bring this new node up to date" action.
func SyncNodes(ctx context.Context, nodeIDs []uint64, scope Scope) (*Summary, error) {
	if scope.IsEmpty() {
		return nil, ErrEmptyScope
	}

	nodes, err := resolveNodes(nodeIDs)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrNoTargetNode
	}

	items, err := buildItems(scope, nil)
	if err != nil {
		return nil, err
	}

	return run(ctx, nodes, items), nil
}

// SyncNamespace replicates the namespace record and every site and stream that
// belongs to it onto all of its member nodes, so each node ends up with the same
// namespace content.
func SyncNamespace(ctx context.Context, namespaceID uint64) (*Summary, error) {
	n := query.Namespace
	namespace, err := n.Where(n.ID.Eq(namespaceID)).First()
	if err != nil {
		return nil, err
	}

	nodes, err := resolveNodes(namespace.SyncNodeIds)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrNamespaceHasNoNode
	}

	items, err := buildItems(Scope{Sites: true, Streams: true, Overwrite: true}, namespace)
	if err != nil {
		return nil, err
	}

	items = append([]item{namespaceItem(namespace)}, items...)

	return run(ctx, nodes, items), nil
}

// buildItems collects the local content matching the scope. When namespace is
// set only its members are collected and they are tagged with its name.
func buildItems(scope Scope, namespace *model.Namespace) ([]item, error) {
	var items []item

	if scope.Configs {
		files, err := CollectConfigFiles(nginx.GetConfPath())
		if err != nil {
			return nil, err
		}
		if len(files) > 0 {
			items = append(items, configBatchItem(fmt.Sprintf("/ (%d)", len(files)), files, scope.Overwrite))
		}
	}

	if scope.Sites {
		siteItems, err := collectSiteItems(namespace, scope.Overwrite)
		if err != nil {
			return nil, err
		}
		items = append(items, siteItems...)
	}

	if scope.Streams {
		streamItems, err := collectStreamItems(namespace, scope.Overwrite)
		if err != nil {
			return nil, err
		}
		items = append(items, streamItems...)
	}

	return items, nil
}

func collectSiteItems(namespace *model.Namespace, overwrite bool) ([]item, error) {
	s := query.Site
	stmt := s.Preload(s.Namespace)
	if namespace != nil {
		stmt = stmt.Where(s.NamespaceID.Eq(namespace.ID))
	}

	sites, err := stmt.Find()
	if err != nil {
		return nil, err
	}

	items := make([]item, 0, len(sites))
	for _, siteModel := range sites {
		content, err := os.ReadFile(siteModel.Path)
		if err != nil {
			logger.Debugf("cluster sync skips unreadable site %s: %v", siteModel.Path, err)
			continue
		}

		name := filepath.Base(siteModel.Path)
		items = append(items, siteItem(
			name,
			string(content),
			namespaceName(lo.Ternary(namespace != nil, namespace, siteModel.Namespace)),
			postSyncAction(siteModel.Namespace),
			siteEnabled(siteModel),
			overwrite,
		))
	}

	return items, nil
}

func collectStreamItems(namespace *model.Namespace, overwrite bool) ([]item, error) {
	s := query.Stream
	stmt := s.Preload(s.Namespace)
	if namespace != nil {
		stmt = stmt.Where(s.NamespaceID.Eq(namespace.ID))
	}

	streams, err := stmt.Find()
	if err != nil {
		return nil, err
	}

	items := make([]item, 0, len(streams))
	for _, streamModel := range streams {
		content, err := os.ReadFile(streamModel.Path)
		if err != nil {
			logger.Debugf("cluster sync skips unreadable stream %s: %v", streamModel.Path, err)
			continue
		}

		name := filepath.Base(streamModel.Path)
		items = append(items, streamItem(
			name,
			string(content),
			namespaceName(lo.Ternary(namespace != nil, namespace, streamModel.Namespace)),
			postSyncAction(streamModel.Namespace),
			streamEnabled(streamModel),
			overwrite,
		))
	}

	return items, nil
}

func namespaceName(namespace *model.Namespace) string {
	if namespace == nil {
		return ""
	}
	return namespace.Name
}

func postSyncAction(namespace *model.Namespace) string {
	if namespace == nil || namespace.PostSyncAction == "" {
		return model.PostSyncActionReloadNginx
	}
	return namespace.PostSyncAction
}

// siteEnabled reports the deployment intent: remote namespaces keep it in the
// database, local ones in the sites-enabled directory.
func siteEnabled(siteModel *model.Site) bool {
	if siteModel.Namespace.IsRemoteDeploy() {
		return siteModel.RemoteEnabled
	}

	return enabledLinkExists("sites-enabled", filepath.Base(siteModel.Path))
}

func streamEnabled(streamModel *model.Stream) bool {
	if streamModel.Namespace.IsRemoteDeploy() {
		return streamModel.RemoteEnabled
	}

	return enabledLinkExists("streams-enabled", filepath.Base(streamModel.Path))
}

func enabledLinkExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(nginx.GetConfPath(dir), name))
	return err == nil
}

// ResolveNamespaceIDByName maps a namespace name onto a local namespace id,
// creating the namespace when the node does not know it yet. It is used by the
// receiving side of a sync where namespace ids differ between nodes.
func ResolveNamespaceIDByName(name string) uint64 {
	if name == "" {
		return 0
	}

	n := query.Namespace
	namespace, err := n.Where(n.Name.Eq(name)).FirstOrCreate()
	if err != nil {
		logger.Errorf("cluster sync could not resolve namespace %s: %v", name, err)
		return 0
	}

	return namespace.ID
}
