package cron

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// setupAutoCertJob initializes the automatic certificate renewal job
func setupAutoCertJob(scheduler gocron.Scheduler) (gocron.Job, error) {
	job, err := scheduler.NewJob(gocron.DurationJob(30*time.Minute),
		gocron.NewTask(cert.AutoCert),
		gocron.WithSingletonMode(gocron.LimitModeWait),
		gocron.JobOption(gocron.WithStartImmediately()))
	if err != nil {
		logger.Errorf("AutoCert Job: Err: %v\n", err)
		return nil, err
	}
	return job, nil
}

// setupCertExpiredJob initializes the certificate expiration check job
func setupCertExpiredJob(scheduler gocron.Scheduler) (gocron.Job, error) {
	job, err := scheduler.NewJob(gocron.DurationJob(6*time.Hour),
		gocron.NewTask(cert.CertExpiredNotify),
		gocron.WithSingletonMode(gocron.LimitModeWait),
		gocron.JobOption(gocron.WithStartImmediately()))
	if err != nil {
		logger.Errorf("CertExpired Job: Err: %v\n", err)
		return nil, err
	}
	return job, nil
}
