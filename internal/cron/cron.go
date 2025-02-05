package cron

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/logrotate"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

var s gocron.Scheduler

func init() {
	var err error
	s, err = gocron.NewScheduler()
	if err != nil {
		logger.Fatalf("Init Scheduler: %v\n", err)
	}
}

var logrotateJob gocron.Job

func InitCronJobs() {
	_, err := s.NewJob(gocron.DurationJob(30*time.Minute),
		gocron.NewTask(cert.AutoCert),
		gocron.WithSingletonMode(gocron.LimitModeWait),
		gocron.JobOption(gocron.WithStartImmediately()))
	if err != nil {
		logger.Fatalf("AutoCert Err: %v\n", err)
	}

	startLogrotate()
	cleanExpiredAuthToken()

	s.Start()
}

func RestartLogrotate() {
	logger.Debug("Restart Logrotate")
	if logrotateJob != nil {
		err := s.RemoveJob(logrotateJob.ID())
		if err != nil {
			logger.Error(err)
			return
		}
	}

	startLogrotate()
}

func startLogrotate() {
	if !settings.LogrotateSettings.Enabled {
		return
	}
	var err error
	logrotateJob, err = s.NewJob(
		gocron.DurationJob(time.Duration(settings.LogrotateSettings.Interval)*time.Minute),
		gocron.NewTask(logrotate.Exec),
		gocron.WithSingletonMode(gocron.LimitModeWait))
	if err != nil {
		logger.Fatalf("LogRotate Job: Err: %v\n", err)
	}
}

func cleanExpiredAuthToken() {
	_, err := s.NewJob(gocron.DurationJob(5*time.Minute), gocron.NewTask(func() {
		logger.Debug("clean expired auth tokens")
		q := query.AuthToken
		_, _ = q.Where(q.ExpiredAt.Lt(time.Now().Unix())).Delete()
	}), gocron.WithSingletonMode(gocron.LimitModeWait))

	if err != nil {
		logger.Fatalf("CleanExpiredAuthToken Err: %v\n", err)
	}
}
