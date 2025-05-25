package cron

import (
	"fmt"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

var (
	autoBackupJobs = make(map[uint64]gocron.Job)
	autoBackupMu   sync.RWMutex
)

// setupAutoBackupJobs initializes all auto backup jobs from database
func setupAutoBackupJobs(s gocron.Scheduler) error {
	autoBackups, err := backup.GetEnabledAutoBackups()
	if err != nil {
		return fmt.Errorf("failed to get enabled auto backups: %w", err)
	}

	for _, autoBackup := range autoBackups {
		err := addAutoBackupJob(s, autoBackup.ID, autoBackup.CronExpression)
		if err != nil {
			logger.Errorf("Failed to add auto backup job for %s: %v", autoBackup.Name, err)
		}
	}

	return nil
}

// addAutoBackupJob adds a single auto backup job to the scheduler
func addAutoBackupJob(s gocron.Scheduler, backupID uint64, cronExpression string) error {
	autoBackupMu.Lock()
	defer autoBackupMu.Unlock()

	// Remove existing job if it exists
	if existingJob, exists := autoBackupJobs[backupID]; exists {
		err := s.RemoveJob(existingJob.ID())
		if err != nil {
			logger.Errorf("Failed to remove existing auto backup job %d: %v", backupID, err)
		}
		delete(autoBackupJobs, backupID)
	}

	// Create new job
	job, err := s.NewJob(
		gocron.CronJob(cronExpression, false),
		gocron.NewTask(executeAutoBackupTask, backupID),
		gocron.WithName(fmt.Sprintf("auto_backup_%d", backupID)),
	)
	if err != nil {
		return fmt.Errorf("failed to create auto backup job: %w", err)
	}

	autoBackupJobs[backupID] = job
	logger.Infof("Added auto backup job %d with cron expression: %s", backupID, cronExpression)
	return nil
}

// removeAutoBackupJob removes an auto backup job from the scheduler
func removeAutoBackupJob(s gocron.Scheduler, backupID uint64) error {
	autoBackupMu.Lock()
	defer autoBackupMu.Unlock()

	if job, exists := autoBackupJobs[backupID]; exists {
		err := s.RemoveJob(job.ID())
		if err != nil {
			return fmt.Errorf("failed to remove auto backup job: %w", err)
		}
		delete(autoBackupJobs, backupID)
		logger.Infof("Removed auto backup job %d", backupID)
	}

	return nil
}

// executeAutoBackupTask executes a single auto backup task
func executeAutoBackupTask(backupID uint64) {
	logger.Infof("Executing auto backup task %d", backupID)

	// Get backup configuration
	autoBackup, err := backup.GetAutoBackupByID(backupID)
	if err != nil {
		logger.Errorf("Failed to get auto backup configuration %d: %v", backupID, err)
		return
	}

	// Check if backup is still enabled
	if !autoBackup.Enabled {
		removeAutoBackupJob(s, backupID)
		logger.Infof("Auto backup %d is disabled, skipping execution", backupID)
		return
	}

	// Execute backup
	err = backup.ExecuteAutoBackup(autoBackup)
	if err != nil {
		logger.Errorf("Auto backup task %d failed: %v", backupID, err)
	} else {
		logger.Infof("Auto backup task %d completed successfully", backupID)
	}
}

// RestartAutoBackupJobs restarts all auto backup jobs
func RestartAutoBackupJobs() error {
	logger.Info("Restarting auto backup jobs...")

	autoBackupMu.Lock()
	defer autoBackupMu.Unlock()

	// Remove all existing jobs
	for backupID, job := range autoBackupJobs {
		err := s.RemoveJob(job.ID())
		if err != nil {
			logger.Errorf("Failed to remove auto backup job %d: %v", backupID, err)
		}
	}
	autoBackupJobs = make(map[uint64]gocron.Job)

	// Re-add all enabled jobs
	err := setupAutoBackupJobs(s)
	if err != nil {
		return fmt.Errorf("failed to restart auto backup jobs: %w", err)
	}

	logger.Info("Auto backup jobs restarted successfully")
	return nil
}

// AddAutoBackupJob adds or updates an auto backup job (public API)
func AddAutoBackupJob(backupID uint64, cronExpression string) error {
	return addAutoBackupJob(s, backupID, cronExpression)
}

// RemoveAutoBackupJob removes an auto backup job (public API)
func RemoveAutoBackupJob(backupID uint64) error {
	return removeAutoBackupJob(s, backupID)
}

// UpdateAutoBackupJob updates an auto backup job (public API)
func UpdateAutoBackupJob(backupID uint64, cronExpression string) error {
	return addAutoBackupJob(s, backupID, cronExpression)
}
