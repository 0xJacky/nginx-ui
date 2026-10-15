package ingress

import "context"

// ReconcileAll compares every ingress with its files on disk, enqueues the
// ones that drifted or are missing, and refreshes certificate and backend
// health in their status.
func ReconcileAll(ctx context.Context) error {
	return ErrNotImplemented
}

func startReconcile(ctx context.Context) {}
