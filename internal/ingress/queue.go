package ingress

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
)

// Enqueue asks the worker to apply the named ingresses. Calls that arrive
// close together are coalesced into one apply.
func Enqueue(names ...string) {}

// WaitForGeneration blocks until the ingress has been applied at least at the
// given generation (successfully or not), or ctx ends.
func WaitForGeneration(ctx context.Context, name string, generation int64) (*model.Ingress, error) {
	return nil, ErrNotImplemented
}

func startWorker(ctx context.Context) {}
