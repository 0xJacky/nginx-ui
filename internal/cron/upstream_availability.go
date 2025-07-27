package cron

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// upstreamAvailabilityJob holds the job instance
var upstreamAvailabilityJob gocron.Job

// setupUpstreamAvailabilityJob initializes the upstream availability testing job
func setupUpstreamAvailabilityJob(scheduler gocron.Scheduler) (gocron.Job, error) {
	job, err := scheduler.NewJob(
		gocron.DurationJob(30*time.Second),
		gocron.NewTask(executeUpstreamAvailabilityTest),
		gocron.WithSingletonMode(gocron.LimitModeWait),
		gocron.WithName("upstream_availability_test"),
		gocron.JobOption(gocron.WithStartImmediately()),
	)
	if err != nil {
		logger.Errorf("UpstreamAvailability Job: Err: %v\n", err)
		return nil, err
	}

	upstreamAvailabilityJob = job
	logger.Info("Upstream availability testing job started with 30s interval")
	return job, nil
}

// executeUpstreamAvailabilityTest performs the upstream availability test
func executeUpstreamAvailabilityTest() {
	service := upstream.GetUpstreamService()

	targetCount := service.GetTargetCount()
	if targetCount == 0 {
		logger.Debug("No upstream targets to test")
		return
	}

	start := time.Now()
	logger.Debug("Starting scheduled upstream availability test for", targetCount, "targets")

	service.PerformAvailabilityTest()

	duration := time.Since(start)
	logger.Debug("Upstream availability test completed in", duration)
}

// RestartUpstreamAvailabilityJob restarts the upstream availability job
func RestartUpstreamAvailabilityJob() error {
	logger.Info("Restarting upstream availability job...")

	// Remove existing job if it exists
	if upstreamAvailabilityJob != nil {
		err := s.RemoveJob(upstreamAvailabilityJob.ID())
		if err != nil {
			logger.Error("Failed to remove existing upstream availability job:", err)
		}
		upstreamAvailabilityJob = nil
	}

	// Create new job
	job, err := setupUpstreamAvailabilityJob(s)
	if err != nil {
		return err
	}

	upstreamAvailabilityJob = job
	logger.Info("Upstream availability job restarted successfully")
	return nil
}
