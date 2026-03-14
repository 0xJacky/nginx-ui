package cron

import (
	"github.com/0xJacky/Nginx-UI/internal/logrotate"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// logrotate job instance
var logrotateJobInstance gocron.Job

// setupLogrotateJob initializes and starts the logrotate job
func setupLogrotateJob(scheduler gocron.Scheduler) {
	if !settings.LogrotateSettings.Enabled {
		return
	}
	if !settings.LogrotateSettings.HasValidInterval() {
		logger.Warnf("Skip logrotate job: %s, got %d",
			settings.InvalidLogrotateIntervalMessage, settings.LogrotateSettings.Interval)
		logrotateJobInstance = nil
		return
	}
	var err error
	logrotateJobInstance, err = scheduler.NewJob(
		gocron.DurationJob(settings.LogrotateSettings.GetInterval()),
		gocron.NewTask(logrotate.Exec),
		gocron.WithSingletonMode(gocron.LimitModeWait))
	if err != nil {
		logger.Errorf("LogRotate Job: Err: %v", err)
		logrotateJobInstance = nil
	}
}

// restartLogrotateJob stops and restarts the logrotate job
func restartLogrotateJob(scheduler gocron.Scheduler) {
	logger.Debug("Restart Logrotate")
	if logrotateJobInstance != nil {
		err := scheduler.RemoveJob(logrotateJobInstance.ID())
		if err != nil {
			logger.Error(err)
			return
		}
	}

	setupLogrotateJob(scheduler)
}
