package cron

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
)

func TestSetupLogrotateJobSkipsInvalidInterval(t *testing.T) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	original := *settings.LogrotateSettings
	t.Cleanup(func() {
		*settings.LogrotateSettings = original
		logrotateJobInstance = nil
		scheduler.Shutdown()
	})

	settings.LogrotateSettings.Enabled = true
	settings.LogrotateSettings.Interval = -1

	setupLogrotateJob(scheduler)

	if logrotateJobInstance != nil {
		t.Fatalf("expected invalid interval to skip job creation")
	}
}

func TestSetupLogrotateJobCreatesJobForValidInterval(t *testing.T) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	original := *settings.LogrotateSettings
	t.Cleanup(func() {
		*settings.LogrotateSettings = original
		logrotateJobInstance = nil
		scheduler.Shutdown()
	})

	settings.LogrotateSettings.Enabled = true
	settings.LogrotateSettings.Interval = 1

	setupLogrotateJob(scheduler)

	if logrotateJobInstance == nil {
		t.Fatalf("expected valid interval to create a job")
	}
}
