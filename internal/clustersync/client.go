package clustersync

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
)

// requestTimeout bounds a single node request. Configuration payloads are small
// text files, so a slow node must not stall the whole run.
const requestTimeout = 30 * time.Second

// nodeRef carries the identity used in results next to the request client.
type nodeRef struct {
	id     uint64
	name   string
	client *resty.Client
}

func newNodeRef(node *model.Node) nodeRef {
	client := nodeauth.NewRestyClient(node)
	client.SetBaseURL(node.URL)
	client.SetTimeout(requestTimeout)

	return nodeRef{id: node.ID, name: node.Name, client: client}
}

// resolveNodes loads the enabled nodes for the given ids, preserving the caller
// order and dropping unknown or disabled ones.
func resolveNodes(nodeIDs []uint64) ([]nodeRef, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}

	n := query.Node
	nodes, err := n.Where(n.ID.In(nodeIDs...), n.Enabled.Is(true)).Find()
	if err != nil {
		return nil, err
	}

	refs := make([]nodeRef, 0, len(nodes))
	for _, node := range nodes {
		refs = append(refs, newNodeRef(node))
	}

	return refs, nil
}

// post sends a JSON body to the node and turns a non-2xx answer into an error.
func (n nodeRef) post(ctx context.Context, path string, body any) error {
	resp, err := n.client.R().SetContext(ctx).SetBody(body).Post(path)
	if err != nil {
		return err
	}
	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return fmt.Errorf("%s responded %d: %s", path, resp.StatusCode(), resp.String())
	}

	return nil
}

// postWithStatus behaves like post but also reports the status code so callers
// can detect endpoints that an older node does not implement yet.
func (n nodeRef) postWithStatus(ctx context.Context, path string, body any) (int, error) {
	_, status, err := n.postForBody(ctx, path, body)
	return status, err
}

// postForBody returns the response body so callers can inspect a partial
// success reported inside a 2xx answer.
func (n nodeRef) postForBody(ctx context.Context, path string, body any) ([]byte, int, error) {
	resp, err := n.client.R().SetContext(ctx).SetBody(body).Post(path)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return resp.Body(), resp.StatusCode(), fmt.Errorf("%s responded %d: %s", path, resp.StatusCode(), resp.String())
	}

	return resp.Body(), resp.StatusCode(), nil
}
