package cron

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/uozi-tech/cosy/logger"
	"github.com/0xJacky/Nginx-UI/internal/logrotate"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron"
	"time"
)

var s *gocron.Scheduler

func init() {
	s = gocron.NewScheduler(time.UTC)
}

var logrotateJob *gocron.Job

func InitCronJobs() {
	job, err := s.Every(30).Minute().SingletonMode().Do(cert.AutoCert)

	if err != nil {
		logger.Fatalf("AutoCert Job: %v, Err: %v\n", job, err)
	}

	startLogrotate()
	cleanExpiredAuthToken()

	s.StartAsync()
}

func RestartLogrotate() {
	logger.Debug("Restart Logrotate")
	if logrotateJob != nil {
		s.RemoveByReference(logrotateJob)
	}

	startLogrotate()
}

func startLogrotate() {
	if !settings.LogrotateSettings.Enabled {
		return
	}
	var err error
	logrotateJob, err = s.Every(settings.LogrotateSettings.Interval).Minute().SingletonMode().Do(logrotate.Exec)
	if err != nil {
		logger.Fatalf("LogRotate Job: %v, Err: %v\n", logrotateJob, err)
	}
}

func cleanExpiredAuthToken() {
	job, err := s.Every(5).Minute().SingletonMode().Do(func() {
		logger.Info("clean expired auth tokens")
		q := query.AuthToken
		_, _ = q.Where(q.ExpiredAt.Lt(time.Now().Unix())).Delete()
	})

	if err != nil {
		logger.Fatalf("CleanExpiredAuthToken Job: %v, Err: %v\n", job, err)
	}
}
