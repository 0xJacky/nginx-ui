package cron

import (
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Global scheduler instance
var s gocron.Scheduler

func init() {
	var err error
	s, err = gocron.NewScheduler()
	if err != nil {
		logger.Fatalf("Init Scheduler: %v\n", err)
	}
}

// InitCronJobs initializes and starts all cron jobs
func InitCronJobs() {
	// Initialize auto cert job
	_, err := setupAutoCertJob(s)
	if err != nil {
		logger.Fatalf("AutoCert Err: %v\n", err)
	}

	// Initialize certificate expiration check job
	_, err = setupCertExpiredJob(s)
	if err != nil {
		logger.Fatalf("CertExpired Err: %v\n", err)
	}

	// Start logrotate job
	setupLogrotateJob(s)

	// Initialize auth token cleanup job
	_, err = setupAuthTokenCleanupJob(s)
	if err != nil {
		logger.Fatalf("CleanExpiredAuthToken Err: %v\n", err)
	}

	// Start the scheduler
	s.Start()
}

// RestartLogrotate is a public API to restart the logrotate job
func RestartLogrotate() {
	restartLogrotateJob(s)
}
