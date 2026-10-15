package ingress

import "context"

// Start runs the apply worker and the drift reconcile loop until ctx ends.
func Start(ctx context.Context) {
	SetCertManager(newCertManager())
	go startWorker(ctx)
	go startReconcile(ctx)
	<-ctx.Done()
}

// RunWorker runs only the apply worker until ctx ends, for callers that
// drive the ingress API without the reconcile loop, such as API tests. It
// returns once the worker and its background health refreshes stopped.
func RunWorker(ctx context.Context) {
	startWorker(ctx)
}
