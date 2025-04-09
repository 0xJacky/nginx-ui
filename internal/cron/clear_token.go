package cron

import (
	"time"

	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// setupAuthTokenCleanupJob initializes the job to clean expired auth tokens
func setupAuthTokenCleanupJob(scheduler gocron.Scheduler) (gocron.Job, error) {
	job, err := scheduler.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(func() {
			logger.Debug("clean expired auth tokens")
			q := query.AuthToken
			_, _ = q.Where(q.ExpiredAt.Lt(time.Now().Unix())).Delete()
		}),
		gocron.WithSingletonMode(gocron.LimitModeWait),
		gocron.JobOption(gocron.WithStartImmediately()))

	if err != nil {
		logger.Errorf("CleanExpiredAuthToken Err: %v\n", err)
		return nil, err
	}

	return job, nil
}
