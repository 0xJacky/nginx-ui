package clustersync

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// configBatchPayload is the wire format understood by the batch receiver. Nodes
// applying it write every file first and reload Nginx once.
type configBatchPayload struct {
	Files     []ConfigFile `json:"files"`
	Overwrite bool         `json:"overwrite"`
}

// sitePayload mirrors the site save endpoint. Namespace carries the group name
// so the receiving node files the site under the same namespace, creating it
// when needed.
type sitePayload struct {
	Content    string `json:"content"`
	Overwrite  bool   `json:"overwrite"`
	PostAction string `json:"post_action"`
	Namespace  string `json:"namespace,omitempty"`
}

// namespacePayload replicates the namespace definition itself. Node membership
// and deploy mode stay controller specific and are deliberately not sent.
type namespacePayload struct {
	Name             string `json:"name"`
	PostSyncAction   string `json:"post_sync_action,omitempty"`
	UpstreamTestType string `json:"upstream_test_type,omitempty"`
}

// configBatchItem replicates a set of configuration files in a single request.
func configBatchItem(name string, files []ConfigFile, overwrite bool) item {
	return item{
		kind: KindConfig,
		name: name,
		push: func(ctx context.Context, node nodeRef) error {
			payload := configBatchPayload{Files: files, Overwrite: overwrite}
			status, err := node.postWithStatus(ctx, "/api/config_sync_batch", payload)
			if err == nil {
				return nil
			}
			if status != http.StatusNotFound {
				return err
			}

			// The node predates the batch receiver, fall back to file by file.
			return pushConfigFilesIndividually(ctx, node, files, overwrite)
		},
	}
}

// pushConfigFilesIndividually keeps mixed-version clusters working by using the
// long-standing single file endpoint.
func pushConfigFilesIndividually(ctx context.Context, node nodeRef, files []ConfigFile, overwrite bool) error {
	var failures []error
	for _, file := range files {
		body := map[string]any{
			"name":      file.Name,
			"base_dir":  file.BaseDir,
			"content":   file.Content,
			"overwrite": overwrite,
		}
		if err := node.post(ctx, "/api/configs", body); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", file.RelativePath(), err))
		}
	}

	return errors.Join(failures...)
}

// siteItem replicates one site together with its enabled state.
func siteItem(name, content, namespace, postAction string, enabled, overwrite bool) item {
	return item{
		kind: KindSite,
		name: name,
		push: func(ctx context.Context, node nodeRef) error {
			payload := sitePayload{
				Content:    content,
				Overwrite:  overwrite,
				PostAction: postAction,
				Namespace:  namespace,
			}
			if err := node.post(ctx, "/api/sites/"+name, payload); err != nil {
				return err
			}

			return applyEnabledState(ctx, node, "/api/sites/"+name, enabled)
		},
	}
}

// streamItem replicates one stream together with its enabled state.
func streamItem(name, content, namespace, postAction string, enabled, overwrite bool) item {
	return item{
		kind: KindStream,
		name: name,
		push: func(ctx context.Context, node nodeRef) error {
			payload := sitePayload{
				Content:    content,
				Overwrite:  overwrite,
				PostAction: postAction,
				Namespace:  namespace,
			}
			if err := node.post(ctx, "/api/streams/"+name, payload); err != nil {
				return err
			}

			return applyEnabledState(ctx, node, "/api/streams/"+name, enabled)
		},
	}
}

// namespaceItem replicates the namespace record so every node groups the synced
// content the same way.
func namespaceItem(namespace *model.Namespace) item {
	return item{
		kind: KindNamespace,
		name: namespace.Name,
		push: func(ctx context.Context, node nodeRef) error {
			payload := namespacePayload{
				Name:             namespace.Name,
				PostSyncAction:   namespace.PostSyncAction,
				UpstreamTestType: namespace.UpstreamTestType,
			}
			status, err := node.postWithStatus(ctx, "/api/namespace/sync", payload)
			if err != nil && status == http.StatusNotFound {
				// Older nodes cannot mirror namespaces; the content still syncs.
				return nil
			}

			return err
		},
	}
}

func applyEnabledState(ctx context.Context, node nodeRef, basePath string, enabled bool) error {
	if enabled {
		return node.post(ctx, basePath+"/enable", nil)
	}

	// Disabling is best effort: a node that never enabled the configuration is
	// already in the desired state and older nodes answer with an error there.
	if err := node.post(ctx, basePath+"/disable", nil); err != nil {
		logger.Debugf("cluster sync could not disable %s on %s: %v", basePath, node.name, err)
	}

	return nil
}
