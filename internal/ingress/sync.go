package ingress

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-resty/resty/v2"
	"github.com/uozi-tech/cosy/logger"
)

// syncToNodes replicates an applied ingress to the sync nodes of its
// namespace; syncRemoveToNodes replicates a removal. They are variables so
// tests can observe them without a remote node.
var (
	syncToNodes       = func(ctx context.Context, item *preparedItem) { dispatchNodeSync(ctx, item, false) }
	syncRemoveToNodes = func(ctx context.Context, item *preparedItem) { dispatchNodeSync(ctx, item, true) }
)

// dispatchNodeSync pushes the generated files to every sync node of the
// namespace through the regular config and site endpoints, one node per
// goroutine. On each node the upstream files go first, so the site never
// references an upstream the node does not have, and they are deleted last.
func dispatchNodeSync(ctx context.Context, item *preparedItem, remove bool) {
	if item.namespace == nil || len(item.namespace.SyncNodeIds) == 0 || nodeauth.IsReplicated(ctx) {
		return
	}
	var nodes []*model.Node
	if err := model.UseDB().Where("id IN ? AND enabled = ?", item.namespace.SyncNodeIds, true).Find(&nodes).Error; err != nil {
		logger.Errorf("Loading sync nodes of ingress %s failed: %v", item.ing.Name, err)
		return
	}

	siteName := SiteName(item.ing.Name)
	upstreamNames := make([]string, 0, len(item.upstreams))
	for name := range item.upstreams {
		upstreamNames = append(upstreamNames, name)
	}
	sort.Strings(upstreamNames)

	for _, node := range nodes {
		go func(node *model.Node) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			var err error
			if remove {
				err = removeOnNode(client, siteName, item, upstreamNames)
			} else {
				err = applyOnNode(client, siteName, item, upstreamNames)
			}
			if err != nil {
				logger.Errorf("Syncing ingress %s to node %s failed: %v", item.ing.Name, node.Name, err)
				return
			}
			logger.Infof("Synced ingress %s to node %s", item.ing.Name, node.Name)
		}(node)
	}
}

func applyOnNode(client *resty.Client, siteName string, item *preparedItem, upstreamNames []string) error {
	for _, name := range upstreamNames {
		path := item.upstreams[name]
		if err := expectOK(client.R().SetBody(map[string]any{
			"name":      filepath.Base(path),
			"base_dir":  relativeConfDir(path),
			"content":   item.rendered.Upstreams[name],
			"overwrite": true,
		}).Post("/api/configs")); err != nil {
			return fmt.Errorf("upstream %s: %w", name, err)
		}
	}
	if err := expectOK(client.R().SetBody(map[string]any{
		"content":     item.rendered.SiteContent,
		"overwrite":   true,
		"post_action": item.namespace.PostSyncAction,
		"namespace":   item.namespace.Name,
	}).Post("/api/sites/" + siteName)); err != nil {
		return fmt.Errorf("site: %w", err)
	}
	if err := expectOK(client.R().Post("/api/sites/" + siteName + "/enable")); err != nil {
		return fmt.Errorf("enable site: %w", err)
	}
	return nil
}

func removeOnNode(client *resty.Client, siteName string, item *preparedItem, upstreamNames []string) error {
	if err := expectOK(client.R().Post("/api/sites/" + siteName + "/disable")); err != nil {
		return fmt.Errorf("disable site: %w", err)
	}
	if err := expectOK(client.R().Delete("/api/sites/" + siteName)); err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	for _, name := range upstreamNames {
		path := item.upstreams[name]
		if err := expectOK(client.R().SetBody(map[string]any{
			"base_path": relativeConfDir(path),
			"name":      filepath.Base(path),
		}).Post("/api/config_delete")); err != nil {
			return fmt.Errorf("upstream %s: %w", name, err)
		}
	}
	return nil
}

// relativeConfDir returns the directory of path relative to the nginx
// configuration root, the form the config endpoints expect.
func relativeConfDir(path string) string {
	return strings.ReplaceAll(filepath.Dir(path), nginx.GetConfPath(), "")
}

func expectOK(resp *resty.Response, err error) error {
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode(), strings.TrimSpace(string(resp.Body())))
	}
	return nil
}
