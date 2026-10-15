package ingress

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
)

// Apply renders the given ingresses and writes their files under one apply
// lock and file transaction, enabling each site, then tests and reloads nginx
// once. When the test fails every file is rolled back. It does not touch the
// database; the caller records the results in each ingress status.
func Apply(ctx context.Context, items []*model.Ingress, opts ApplyOptions) ([]ApplyResult, error) {
	return nil, ErrNotImplemented
}

// Remove deletes the generated files of the given ingresses (enabled link,
// site file, upstream files) and the generated site rows, then tests and
// reloads nginx once.
func Remove(ctx context.Context, items []*model.Ingress) error {
	return ErrNotImplemented
}
