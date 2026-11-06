package cron

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jobRunTimeout only bounds a hung run so a broken test fails instead of
// blocking; passing runs are signalled through a channel immediately.
const jobRunTimeout = 10 * time.Second

// newRunningScheduler returns a scheduler and a channel that receives the ID
// of every job run as soon as it finishes. The caller starts the scheduler.
func newRunningScheduler(t *testing.T) (gocron.Scheduler, <-chan uuid.UUID) {
	t.Helper()

	finished := make(chan uuid.UUID, 16)
	scheduler := newTestScheduler(t,
		gocron.WithGlobalJobOptions(gocron.WithEventListeners(
			gocron.AfterJobRuns(func(jobID uuid.UUID, _ string) {
				select {
				case finished <- jobID:
				default:
				}
			}),
		)),
	)
	return scheduler, finished
}

func waitForRun(t *testing.T, finished <-chan uuid.UUID, jobID uuid.UUID) {
	t.Helper()

	timeout := time.After(jobRunTimeout)
	for {
		select {
		case id := <-finished:
			if id == jobID {
				return
			}
		case <-timeout:
			t.Fatalf("job %s did not run", jobID)
		}
	}
}

// waitForNextRun returns the job's next scheduled run once the scheduler has
// recorded the one that follows the run observed at after. gocron dispatches
// an immediate first run before it has scheduled the next one, and its loop
// handles the executor's reschedule, the run's completion (which prunes the
// run that just happened) and the NextRun query in any order. Right after a
// run, NextRun can therefore still report that run, or no run at all (the
// zero time).
func waitForNextRun(t *testing.T, job gocron.Job, after time.Time) time.Time {
	t.Helper()

	deadline := time.Now().Add(jobRunTimeout)
	for {
		next, err := job.NextRun()
		require.NoError(t, err)
		if next.After(after) {
			return next
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s was not rescheduled after its run, next run %s", job.ID(), next)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSetupJobsRegisterOnScheduler(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(gocron.Scheduler) (gocron.Job, error)
		wantName string
	}{
		{name: "auto cert", setup: setupAutoCertJob},
		{name: "cert expired", setup: setupCertExpiredJob},
		{name: "self signed cert renewal", setup: setupSelfSignedCertRenewalJob},
		{name: "auth token cleanup", setup: setupAuthTokenCleanupJob},
		{name: "node credential maintenance", setup: setupNodeCredentialMaintenanceJob, wantName: "node_credential_maintenance"},
		{name: "upstream availability", setup: setupUpstreamAvailabilityJob, wantName: "upstream_availability_test"},
		{name: "namespace sync", setup: setupNamespaceSyncJob, wantName: "namespace_auto_sync"},
	}

	originalUpstreamJob := upstreamAvailabilityJob
	t.Cleanup(func() { upstreamAvailabilityJob = originalUpstreamJob })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The scheduler is never started, so none of the tasks run.
			scheduler := newTestScheduler(t)

			job, err := tt.setup(scheduler)
			require.NoError(t, err)
			require.NotNil(t, job)

			assert.True(t, hasJob(scheduler, job.ID()))
			assert.Len(t, scheduler.Jobs(), 1)
			if tt.wantName != "" {
				assert.Equal(t, tt.wantName, job.Name())
			}
		})
	}
}

func TestIntervalJobsFollowSettings(t *testing.T) {
	originalUpstream := *settings.UpstreamCheckSettings
	originalUpstreamJob := upstreamAvailabilityJob
	t.Cleanup(func() {
		*settings.UpstreamCheckSettings = originalUpstream
		upstreamAvailabilityJob = originalUpstreamJob
	})

	// The task is a no-op here: there are no upstream targets, so running it
	// immediately is harmless.

	tests := []struct {
		name      string
		configure func()
		setup     func(gocron.Scheduler) (gocron.Job, error)
		want      time.Duration
	}{
		{
			name:      "upstream interval in seconds",
			configure: func() { settings.UpstreamCheckSettings.IntervalSeconds = 45 },
			setup:     setupUpstreamAvailabilityJob,
			want:      45 * time.Second,
		},
		{
			name:      "upstream interval below minimum falls back to default",
			configure: func() { settings.UpstreamCheckSettings.IntervalSeconds = 1 },
			setup:     setupUpstreamAvailabilityJob,
			want:      30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.configure()
			scheduler, finished := newRunningScheduler(t)

			job, err := tt.setup(scheduler)
			require.NoError(t, err)
			before := time.Now()
			scheduler.Start()

			// The job starts immediately, then waits one interval, so the
			// next run is one interval after the start.
			waitForRun(t, finished, job.ID())
			after := time.Now()
			next := waitForNextRun(t, job, after)
			assert.False(t, next.Before(before.Add(tt.want)), "next run %s is earlier than one interval after start", next)
			assert.False(t, next.After(after.Add(tt.want)), "next run %s is later than one interval after start", next)
		})
	}
}

func TestAuthTokenCleanupJobDeletesOnlyExpiredTokens(t *testing.T) {
	db := useCronTestDB(t, &model.AuthToken{})

	now := time.Now().Unix()
	tokens := []model.AuthToken{
		{UserID: 1, Token: "expired", ExpiredAt: now - 60},
		{UserID: 1, Token: "long-expired", ExpiredAt: 1},
		{UserID: 2, Token: "valid", ExpiredAt: now + 3600},
	}
	require.NoError(t, db.Create(&tokens).Error)

	scheduler, finished := newRunningScheduler(t)
	job, err := setupAuthTokenCleanupJob(scheduler)
	require.NoError(t, err)
	scheduler.Start()

	waitForRun(t, finished, job.ID())

	var remaining []model.AuthToken
	require.NoError(t, db.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, "valid", remaining[0].Token)
}

func TestRestartUpstreamAvailabilityJobReplacesJobOnGlobalScheduler(t *testing.T) {
	originalJob := upstreamAvailabilityJob
	upstreamAvailabilityJob = nil
	t.Cleanup(func() {
		if upstreamAvailabilityJob != nil {
			_ = s.RemoveJob(upstreamAvailabilityJob.ID())
		}
		upstreamAvailabilityJob = originalJob
	})

	require.NoError(t, RestartUpstreamAvailabilityJob())
	first := upstreamAvailabilityJob
	require.NotNil(t, first)
	assert.True(t, hasJob(s, first.ID()))

	require.NoError(t, RestartUpstreamAvailabilityJob())
	second := upstreamAvailabilityJob
	require.NotNil(t, second)

	assert.NotEqual(t, first.ID(), second.ID())
	assert.False(t, hasJob(s, first.ID()), "previous job must be removed")
	assert.Equal(t, 1, countJobsNamed(s, "upstream_availability_test"))
}

func TestRestartLogrotateJob(t *testing.T) {
	original := *settings.LogrotateSettings
	t.Cleanup(func() {
		*settings.LogrotateSettings = original
		logrotateJobInstance = nil
	})

	t.Run("disabled setting registers nothing", func(t *testing.T) {
		scheduler := newTestScheduler(t)
		logrotateJobInstance = nil
		settings.LogrotateSettings.Enabled = false
		settings.LogrotateSettings.Interval = 60

		restartLogrotateJob(scheduler)

		assert.Nil(t, logrotateJobInstance)
		assert.Empty(t, scheduler.Jobs())
	})

	t.Run("enabled setting replaces the running job", func(t *testing.T) {
		scheduler := newTestScheduler(t)
		logrotateJobInstance = nil
		settings.LogrotateSettings.Enabled = true
		settings.LogrotateSettings.Interval = 60

		restartLogrotateJob(scheduler)
		first := logrotateJobInstance
		require.NotNil(t, first)

		restartLogrotateJob(scheduler)
		second := logrotateJobInstance
		require.NotNil(t, second)

		assert.NotEqual(t, first.ID(), second.ID())
		assert.False(t, hasJob(scheduler, first.ID()))
		assert.True(t, hasJob(scheduler, second.ID()))
		assert.Len(t, scheduler.Jobs(), 1)
	})

	t.Run("disabling removes the job", func(t *testing.T) {
		scheduler := newTestScheduler(t)
		logrotateJobInstance = nil
		settings.LogrotateSettings.Enabled = true
		settings.LogrotateSettings.Interval = 60
		setupLogrotateJob(scheduler)
		require.NotNil(t, logrotateJobInstance)

		settings.LogrotateSettings.Enabled = false
		restartLogrotateJob(scheduler)

		assert.Empty(t, scheduler.Jobs())
	})

	t.Run("re-enabling after disabling schedules the job again", func(t *testing.T) {
		scheduler := newTestScheduler(t)
		logrotateJobInstance = nil
		settings.LogrotateSettings.Enabled = true
		settings.LogrotateSettings.Interval = 60
		restartLogrotateJob(scheduler)
		require.NotNil(t, logrotateJobInstance)

		settings.LogrotateSettings.Enabled = false
		restartLogrotateJob(scheduler)
		require.Empty(t, scheduler.Jobs())
		assert.Nil(t, logrotateJobInstance)

		settings.LogrotateSettings.Enabled = true
		restartLogrotateJob(scheduler)

		require.NotNil(t, logrotateJobInstance)
		assert.Len(t, scheduler.Jobs(), 1)
		assert.True(t, hasJob(scheduler, logrotateJobInstance.ID()))
	})

	t.Run("a job already gone from the scheduler is replaced", func(t *testing.T) {
		scheduler := newTestScheduler(t)
		logrotateJobInstance = nil
		settings.LogrotateSettings.Enabled = true
		settings.LogrotateSettings.Interval = 60
		setupLogrotateJob(scheduler)
		require.NotNil(t, logrotateJobInstance)
		require.NoError(t, scheduler.RemoveJob(logrotateJobInstance.ID()))

		restartLogrotateJob(scheduler)

		require.NotNil(t, logrotateJobInstance)
		assert.Len(t, scheduler.Jobs(), 1)
	})

	t.Run("public API restarts on the global scheduler", func(t *testing.T) {
		logrotateJobInstance = nil
		settings.LogrotateSettings.Enabled = true
		settings.LogrotateSettings.Interval = 60
		t.Cleanup(func() {
			if logrotateJobInstance != nil {
				_ = s.RemoveJob(logrotateJobInstance.ID())
			}
		})

		RestartLogrotate()

		require.NotNil(t, logrotateJobInstance)
		assert.True(t, hasJob(s, logrotateJobInstance.ID()))
	})
}

func TestSetupLogrotateJobSkipsWhenDisabled(t *testing.T) {
	scheduler := newTestScheduler(t)

	original := *settings.LogrotateSettings
	t.Cleanup(func() {
		*settings.LogrotateSettings = original
		logrotateJobInstance = nil
	})

	logrotateJobInstance = nil
	settings.LogrotateSettings.Enabled = false
	settings.LogrotateSettings.Interval = 60

	setupLogrotateJob(scheduler)

	assert.Nil(t, logrotateJobInstance)
	assert.Empty(t, scheduler.Jobs())
}
