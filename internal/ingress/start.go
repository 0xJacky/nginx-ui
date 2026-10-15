package ingress

import "context"

// Start runs the apply worker and the drift reconcile loop until ctx ends.
func Start(ctx context.Context) {
	SetCertManager(newCertManager())
	go startWorker(ctx)
	go startReconcile(ctx)
	<-ctx.Done()
}
