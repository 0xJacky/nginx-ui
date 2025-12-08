package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

var (
	ddnsJobs = make(map[uint64]gocron.Job)
	ddnsMu   sync.RWMutex
)

func setupDDNSJobs(s gocron.Scheduler) error {
	schedules, err := dns.ListEnabledDDNSSchedules(context.Background())
	if err != nil {
		return fmt.Errorf("load ddns schedules: %w", err)
	}

	for _, schedule := range schedules {
		if err := addDDNSJob(s, schedule.DomainID, schedule.IntervalSeconds); err != nil {
			logger.Errorf("Add DDNS job %d failed: %v", schedule.DomainID, err)
		}
	}

	return nil
}

func addDDNSJob(s gocron.Scheduler, domainID uint64, intervalSeconds int) error {
	if intervalSeconds <= 0 {
		return fmt.Errorf("invalid ddns interval for domain %d", domainID)
	}

	ddnsMu.Lock()
	defer ddnsMu.Unlock()

	if job, exists := ddnsJobs[domainID]; exists {
		if err := s.RemoveJob(job.ID()); err != nil {
			logger.Warnf("Remove existing DDNS job %d failed: %v", domainID, err)
		}
		delete(ddnsJobs, domainID)
	}

	job, err := s.NewJob(
		gocron.DurationJob(time.Duration(intervalSeconds)*time.Second),
		gocron.NewTask(executeDDNSJob, domainID),
		gocron.WithName(fmt.Sprintf("ddns_%d", domainID)),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return fmt.Errorf("create ddns job: %w", err)
	}

	ddnsJobs[domainID] = job
	logger.Infof("Added DDNS job %d with interval %ds", domainID, intervalSeconds)
	return nil
}

func removeDDNSJob(s gocron.Scheduler, domainID uint64) error {
	ddnsMu.Lock()
	defer ddnsMu.Unlock()

	if job, exists := ddnsJobs[domainID]; exists {
		if err := s.RemoveJob(job.ID()); err != nil {
			return fmt.Errorf("remove ddns job: %w", err)
		}
		delete(ddnsJobs, domainID)
		logger.Infof("Removed DDNS job %d", domainID)
	}
	return nil
}

// AddOrUpdateDDNSJob adds or replaces a DDNS job using the global scheduler.
func AddOrUpdateDDNSJob(domainID uint64, intervalSeconds int) error {
	return addDDNSJob(s, domainID, intervalSeconds)
}

// RemoveDDNSJob removes a DDNS job from the global scheduler.
func RemoveDDNSJob(domainID uint64) error {
	return removeDDNSJob(s, domainID)
}

func executeDDNSJob(domainID uint64) {
	if err := dns.RunDDNSUpdate(context.Background(), domainID); err != nil {
		logger.Errorf("DDNS job %d failed: %v", domainID, err)
	}
}
