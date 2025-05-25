package backup

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// GetAutoBackupList retrieves a paginated list of auto backup configurations.
// This endpoint supports fuzzy search by backup name and filtering by backup type and enabled status.
//
// Query Parameters:
//   - page: Page number for pagination
//   - page_size: Number of items per page
//   - name: Fuzzy search filter for backup name
//   - backup_type: Filter by backup type (nginx_config/nginx_ui_config/both_config/custom_dir)
//   - enabled: Filter by enabled status (true/false)
//
// Response: Paginated list of auto backup configurations
func GetAutoBackupList(c *gin.Context) {
	cosy.Core[model.AutoBackup](c).
		SetFussy("name").
		SetEqual("backup_type", "enabled", "storage_type", "last_backup_status").
		PagingList()
}

// CreateAutoBackup creates a new auto backup configuration with comprehensive validation.
// This endpoint validates all required fields, path permissions, and S3 configuration.
//
// Request Body: AutoBackup model with required fields
// Response: Created auto backup configuration
func CreateAutoBackup(c *gin.Context) {
	ctx := cosy.Core[model.AutoBackup](c).SetValidRules(gin.H{
		"name":                 "required",
		"backup_type":          "required",
		"storage_type":         "required",
		"storage_path":         "required",
		"cron_expression":      "required",
		"enabled":              "omitempty",
		"backup_path":          "omitempty",
		"s3_endpoint":          "omitempty",
		"s3_access_key_id":     "omitempty",
		"s3_secret_access_key": "omitempty",
		"s3_bucket":            "omitempty",
		"s3_region":            "omitempty",
	}).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Validate backup configuration before creation
		if err := backup.ValidateAutoBackupConfig(&ctx.Model); err != nil {
			ctx.AbortWithError(err)
			return
		}
	})

	ctx.Create()

	// Register cron job only if the backup is enabled
	if ctx.Model.Enabled {
		if err := cron.AddAutoBackupJob(ctx.Model.ID, ctx.Model.CronExpression); err != nil {
			logger.Errorf("Failed to add auto backup job %d: %v", ctx.Model.ID, err)
		}
	}
}

// GetAutoBackup retrieves a single auto backup configuration by ID.
//
// Path Parameters:
//   - id: Auto backup configuration ID
//
// Response: Auto backup configuration details
func GetAutoBackup(c *gin.Context) {
	cosy.Core[model.AutoBackup](c).Get()
}

// ModifyAutoBackup updates an existing auto backup configuration with validation.
// This endpoint performs the same validation as creation for modified fields.
//
// Path Parameters:
//   - id: Auto backup configuration ID
//
// Request Body: Partial AutoBackup model with fields to update
// Response: Updated auto backup configuration
func ModifyAutoBackup(c *gin.Context) {
	ctx := cosy.Core[model.AutoBackup](c).SetValidRules(gin.H{
		"name":                 "omitempty",
		"backup_type":          "omitempty",
		"storage_type":         "omitempty",
		"storage_path":         "omitempty",
		"cron_expression":      "omitempty",
		"backup_path":          "omitempty",
		"enabled":              "omitempty",
		"s3_endpoint":          "omitempty",
		"s3_access_key_id":     "omitempty",
		"s3_secret_access_key": "omitempty",
		"s3_bucket":            "omitempty",
		"s3_region":            "omitempty",
	}).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Validate backup configuration before modification
		if err := backup.ValidateAutoBackupConfig(&ctx.Model); err != nil {
			ctx.AbortWithError(err)
			return
		}
	})

	ctx.Modify()

	// Update cron job based on enabled status
	if ctx.Model.Enabled {
		if err := cron.UpdateAutoBackupJob(ctx.Model.ID, ctx.Model.CronExpression); err != nil {
			logger.Errorf("Failed to update auto backup job %d: %v", ctx.Model.ID, err)
		}
	} else {
		if err := cron.RemoveAutoBackupJob(ctx.Model.ID); err != nil {
			logger.Errorf("Failed to remove auto backup job %d: %v", ctx.Model.ID, err)
		}
	}
}

// DestroyAutoBackup deletes an auto backup configuration and removes its cron job.
// This endpoint ensures proper cleanup of both database records and scheduled tasks.
//
// Path Parameters:
//   - id: Auto backup configuration ID
//
// Response: Success confirmation
func DestroyAutoBackup(c *gin.Context) {
	cosy.Core[model.AutoBackup](c).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Remove cron job before deleting the backup task
		if err := cron.RemoveAutoBackupJob(ctx.Model.ID); err != nil {
			logger.Errorf("Failed to remove auto backup job %d: %v", ctx.Model.ID, err)
		}
	}).Destroy()
}

// TestS3Connection tests the S3 connection for auto backup configuration.
// This endpoint allows users to verify their S3 settings before saving the configuration.
//
// Request Body: AutoBackup model with S3 configuration
// Response: Success confirmation or error details
func TestS3Connection(c *gin.Context) {
	var autoBackup model.AutoBackup
	if !cosy.BindAndValid(c, &autoBackup) {
		return
	}

	// Validate S3 configuration
	if err := backup.ValidateS3Config(&autoBackup); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Test S3 connection
	if err := backup.TestS3ConnectionForConfig(&autoBackup); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "S3 connection test successful"})
}

// RestoreAutoBackup restores a soft-deleted auto backup configuration.
// This endpoint restores the backup configuration and re-registers the cron job if enabled.
//
// Path Parameters:
//   - id: Auto backup configuration ID to restore
//
// Response: Success confirmation
func RestoreAutoBackup(c *gin.Context) {
	var autoBackup model.AutoBackup
	if err := c.ShouldBindUri(&autoBackup); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Restore the backup configuration
	if err := backup.RestoreAutoBackup(autoBackup.ID); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Get the restored backup configuration to check if it's enabled
	restoredBackup, err := backup.GetAutoBackupByID(autoBackup.ID)
	if err != nil {
		logger.Errorf("Failed to get restored auto backup %d: %v", autoBackup.ID, err)
	} else if restoredBackup.Enabled {
		// Register cron job if the backup is enabled
		if err := cron.AddAutoBackupJob(restoredBackup.ID, restoredBackup.CronExpression); err != nil {
			logger.Errorf("Failed to add auto backup job %d after restore: %v", restoredBackup.ID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Auto backup restored successfully"})
}
