package cron

import (
	"context"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// namespaceSyncTick is how often the scheduler looks for namespaces that are due.
// Each namespace defines its own interval, this tick only decides when to check.
const namespaceSyncTick = time.Minute

// namespaceSyncTimeout bounds one namespace replication run.
const namespaceSyncTimeout = 10 * time.Minute

// lastNamespaceSync remembers when each namespace was last replicated so the
// per-namespace interval can be honoured without an extra database column.
var lastNamespaceSync sync.Map

// namespaceSyncNotification is the payload of the auto sync notifications.
type namespaceSyncNotification struct {
	Namespace string `json:"namespace"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
}

// setupNamespaceSyncJob initializes the automatic namespace replication job.
func setupNamespaceSyncJob(scheduler gocron.Scheduler) (gocron.Job, error) {
	return scheduler.NewJob(
		gocron.DurationJob(namespaceSyncTick),
		gocron.NewTask(executeNamespaceAutoSync),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName("namespace_auto_sync"),
	)
}

// executeNamespaceAutoSync replicates every namespace whose interval elapsed.
func executeNamespaceAutoSync() {
	n := query.Namespace
	namespaces, err := n.Where(n.SyncStrategy.Eq(model.SyncStrategyAuto)).Find()
	if err != nil {
		logger.Errorf("NamespaceAutoSync: list namespaces: %v", err)
		return
	}

	now := time.Now()
	for _, namespace := range namespaces {
		if len(namespace.SyncNodeIds) == 0 {
			continue
		}

		interval := time.Duration(namespace.EffectiveSyncInterval()) * time.Minute
		if last, ok := lastNamespaceSync.Load(namespace.ID); ok {
			if now.Sub(last.(time.Time)) < interval {
				continue
			}
		}
		lastNamespaceSync.Store(namespace.ID, now)

		ctx, cancel := context.WithTimeout(context.Background(), namespaceSyncTimeout)
		summary, err := clustersync.SyncNamespace(ctx, namespace.ID)
		cancel()
		if err != nil {
			logger.Errorf("NamespaceAutoSync: %s: %v", namespace.Name, err)
			continue
		}

		payload := namespaceSyncNotification{
			Namespace: namespace.Name,
			Succeeded: summary.Succeeded,
			Failed:    summary.Failed,
		}

		if summary.Failed > 0 {
			notification.Error("Auto Sync Namespace Error",
				"Auto sync of namespace %{namespace} finished with %{failed} failed items", payload)
			continue
		}

		logger.Debugf("NamespaceAutoSync: %s replicated %d items", namespace.Name, summary.Succeeded)
	}
}
