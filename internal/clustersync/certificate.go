package clustersync

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// KindCertificate is a certificate pair pushed through the certificate sync
// endpoint of a node.
const KindCertificate Kind = "certificate"

// certificateItem replicates the certificate pairs a set of sites and streams
// load. It is blocking: a node without the files rejects every site that
// loads them in its Nginx test. Certificates never travel with a plain config
// sync, which only accepts configuration files.
func certificateItem(payloads []*cert.SyncCertificatePayload) (item, bool) {
	if len(payloads) == 0 {
		return item{}, false
	}
	return item{
		kind:     KindCertificate,
		name:     fmt.Sprintf("certificates (%d)", len(payloads)),
		blocking: true,
		push: func(ctx context.Context, node nodeRef) error {
			var failures []error
			for _, payload := range payloads {
				if err := node.put(ctx, "/api/cert_sync", payload); err != nil {
					failures = append(failures, fmt.Errorf("%s: %w", payload.SSLCertificatePath, err))
				}
			}
			return errors.Join(failures...)
		},
	}, true
}

// referencedCertificates collects the certificate pairs the given site and
// stream files load, each pair once.
func referencedCertificates(files []ConfigFile) []*cert.SyncCertificatePayload {
	seen := map[string]bool{}
	var payloads []*cert.SyncCertificatePayload
	for _, file := range files {
		for _, payload := range cert.ReferencedSyncPayloads(file.Content) {
			if seen[payload.SSLCertificatePath] {
				continue
			}
			seen[payload.SSLCertificatePath] = true
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

// PushCertificatesToNodes replicates the certificate pairs a site or stream
// file loads before the file itself is pushed to the nodes. Failures are
// logged: the push that follows reports the resulting Nginx test error.
func PushCertificatesToNodes(content string, nodes []*model.Node) {
	if len(nodes) == 0 {
		return
	}
	pushItem, ok := certificateItem(cert.ReferencedSyncPayloads(content))
	if !ok {
		return
	}

	ids := make([]uint64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	refs, err := resolveNodes(ids)
	if err != nil {
		logger.Errorf("Pushing certificates to remote nodes failed: %v", err)
		return
	}

	summary := run(context.Background(), refs, []item{pushItem})
	if summary.Failed > 0 {
		logger.Errorf("Pushing certificates failed on %d node(s)", summary.Failed)
	}
}

// put sends a JSON body with PUT and turns a non-2xx answer into an error.
func (n nodeRef) put(ctx context.Context, path string, body any) error {
	resp, err := n.client.R().SetContext(ctx).SetBody(body).Put(path)
	if err != nil {
		return err
	}
	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return fmt.Errorf("%s responded %d: %s", path, resp.StatusCode(), resp.String())
	}

	return nil
}
