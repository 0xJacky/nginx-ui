package clustersync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy/logger"
)

// configBatchPayload is the wire format understood by the batch receiver. Nodes
// applying it write every file first and reload Nginx once.
type configBatchPayload struct {
	Async     bool         `json:"async,omitempty"`
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
	SourceID         uint64 `json:"source_id,omitempty"`
	OriginalName     string `json:"original_name,omitempty"`
	Name             string `json:"name"`
	PostSyncAction   string `json:"post_sync_action,omitempty"`
	UpstreamTestType string `json:"upstream_test_type,omitempty"`
}

// configBatchItem replicates a set of configuration files in a single request.
func configBatchItem(name string, files []ConfigFile, overwrite bool) item {
	return newConfigBatchItem(name, files, overwrite, false)
}

// managedConfigBatchItem stages every site and stream file before any of them
// is enabled or validated through its resource endpoint. Nginx validates the
// whole configuration tree, so applying a dependent site one at a time can
// fail solely because its upstream file has not arrived yet.
func managedConfigBatchItem(name string, files []ConfigFile, overwrite bool) item {
	return newConfigBatchItem(name, files, overwrite, true)
}

func newConfigBatchItem(name string, files []ConfigFile, overwrite, blocking bool) item {
	return item{
		kind:     KindConfig,
		name:     name,
		blocking: blocking,
		pushOutcome: func(ctx context.Context, node nodeRef) (outcome, error) {
			payload := configBatchPayload{Async: true, Files: files, Overwrite: overwrite}
			body, status, err := node.postForBody(ctx, "/api/config_sync_batch", payload)
			if status == http.StatusNotFound {
				// The node predates the batch receiver, fall back to file by file.
				return outcome{}, pushConfigFilesIndividually(ctx, node, files, overwrite)
			}
			if err != nil {
				return outcome{}, err
			}
			if status == http.StatusAccepted {
				body, err = waitForBatch(ctx, node, body)
				if err != nil {
					return outcome{}, err
				}
			}
			return batchOutcome(body), batchFailure(body)
		},
	}
}

// waitForBatch polls only the accepted write. Reposting it after a proxy
// timeout could apply or reload the same configuration twice.
func waitForBatch(ctx context.Context, node nodeRef, accepted []byte) ([]byte, error) {
	var task struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(accepted, &task); err != nil || task.ID == uuid.Nil {
		return nil, errors.New("node accepted the configuration batch without a valid task ID")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	path := "/api/config_sync_batch/" + task.ID.String()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		resp, err := node.client.R().SetContext(ctx).Get(path)
		if err != nil {
			return nil, err
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			var result struct {
				Written *int `json:"written"`
			}
			if err := json.Unmarshal(resp.Body(), &result); err != nil || result.Written == nil {
				return nil, fmt.Errorf("%s returned an invalid configuration batch result", path)
			}
			return resp.Body(), nil
		case http.StatusAccepted:
		default:
			return nil, fmt.Errorf("%s responded %d: %s", path, resp.StatusCode(), resp.String())
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// batchResponse is the answer of the batch receiver. Files it could not apply
// are reported individually instead of failing the whole request.
type batchResponse struct {
	Written int `json:"written"`
	Skipped int `json:"skipped"`
	// SkippedPaths is absent on nodes that predate it.
	SkippedPaths []string `json:"skipped_paths"`
	Failures     []struct {
		Path  string `json:"path"`
		Error string `json:"error"`
	} `json:"failures"`
}

// batchOutcome extracts the files the node left alone from a batch answer.
func batchOutcome(body []byte) outcome {
	var response batchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return outcome{}
	}

	count := response.Skipped
	if len(response.SkippedPaths) > count {
		count = len(response.SkippedPaths)
	}

	return outcome{skippedExisting: count, skippedPaths: response.SkippedPaths}
}

// batchFailure turns a partially applied batch into an error so the summary
// does not claim a clean run.
func batchFailure(body []byte) error {
	var response batchResponse
	if err := json.Unmarshal(body, &response); err != nil || len(response.Failures) == 0 {
		return nil
	}

	failures := make([]error, 0, len(response.Failures))
	for _, failure := range response.Failures {
		failures = append(failures, fmt.Errorf("%s: %s", failure.Path, failure.Error))
	}

	return fmt.Errorf("applied %d of %d files: %w",
		response.Written, response.Written+len(response.Failures), errors.Join(failures...))
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
		kind:     KindNamespace,
		blocking: true,
		name:     namespace.Name,
		push: func(ctx context.Context, node nodeRef) error {
			payload := namespacePayload{
				SourceID:         namespace.ID,
				OriginalName:     namespace.LegacyMemberNames[node.id],
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
