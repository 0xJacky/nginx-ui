package cron

import (
	"context"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

func setupNodeCredentialMaintenanceJob(scheduler gocron.Scheduler) (gocron.Job, error) {
	return scheduler.NewJob(
		gocron.DurationJob(time.Hour),
		gocron.NewTask(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			for _, issue := range nodeauth.MaintainRelationships(
				ctx,
				settings.NodeSettings.InstanceID,
				time.Now(),
			) {
				logger.Warnf("Automatic node credential %s failed for node %d: %v", issue.Operation, issue.NodeID, issue.Err)
			}
		}),
		gocron.WithName("node_credential_maintenance"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
}
