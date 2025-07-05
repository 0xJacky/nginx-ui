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

	// Check if we should skip this test due to active WebSocket connections
	// (WebSocket connections trigger more frequent checks)
	if hasActiveWebSocketConnections() {
		logger.Debug("Skipping scheduled test due to active WebSocket connections")
		return
	}

	start := time.Now()
	logger.Debug("Starting scheduled upstream availability test for", targetCount, "targets")

	service.PerformAvailabilityTest()

	duration := time.Since(start)
	logger.Debug("Upstream availability test completed in", duration)
}

// hasActiveWebSocketConnections checks if there are active WebSocket connections
// This is a placeholder - the actual implementation should check the API package
func hasActiveWebSocketConnections() bool {
	// TODO: This should check api/upstream.HasActiveWebSocketConnections()
	// but we need to avoid circular dependencies
	return false
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
