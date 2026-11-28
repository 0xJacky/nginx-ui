package backup

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

type redactedAutoBackup struct {
	model.Model
	Name             string             `json:"name"`
	BackupType       model.BackupType   `json:"backup_type"`
	StorageType      model.StorageType  `json:"storage_type"`
	BackupPath       string             `json:"backup_path"`
	StoragePath      string             `json:"storage_path"`
	CronExpression   string             `json:"cron_expression"`
	Enabled          bool               `json:"enabled"`
	RetentionCount   int                `json:"retention_count"`
	LastBackupTime   *time.Time         `json:"last_backup_time"`
	LastBackupStatus model.BackupStatus `json:"last_backup_status"`
	LastBackupError  string             `json:"last_backup_error"`
	S3Endpoint       string             `json:"s3_endpoint"`
	S3Bucket         string             `json:"s3_bucket"`
	S3Region         string             `json:"s3_region"`
}

func redactAutoBackup(autoBackup *model.AutoBackup) redactedAutoBackup {
	return redactedAutoBackup{
		Model:            autoBackup.Model,
		Name:             autoBackup.Name,
		BackupType:       autoBackup.BackupType,
		StorageType:      autoBackup.StorageType,
		BackupPath:       autoBackup.BackupPath,
		StoragePath:      autoBackup.StoragePath,
		CronExpression:   autoBackup.CronExpression,
		Enabled:          autoBackup.Enabled,
		RetentionCount:   autoBackup.RetentionCount,
		LastBackupTime:   autoBackup.LastBackupTime,
		LastBackupStatus: autoBackup.LastBackupStatus,
		LastBackupError:  autoBackup.LastBackupError,
		S3Endpoint:       autoBackup.S3Endpoint,
		S3Bucket:         autoBackup.S3Bucket,
		S3Region:         autoBackup.S3Region,
	}
}

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
	core := cosy.Core[model.AutoBackup](c).
		SetFussy("name").
		SetEqual("backup_type", "enabled", "storage_type", "last_backup_status")
	if internalmcp.IsServiceTokenRequest(c) {
		core.SetTransformer(redactAutoBackup)
	}
	core.PagingList()
}

// CreateAutoBackup creates a new auto backup configuration with comprehensive validation.
// This endpoint validates all required fields, path permissions, and S3 configuration.
//
// Request Body: AutoBackup model with required fields
// Response: Created auto backup configuration
func CreateAutoBackup(c *gin.Context) {
	cosy.Core[model.AutoBackup](c).SetValidRules(gin.H{
		"name":                 "required",
		"backup_type":          "required",
		"storage_type":         "required",
		"storage_path":         "required",
		"cron_expression":      "required",
		"enabled":              "omitempty",
		"retention_count":      "omitempty,min=0",
		"backup_path":          "omitempty",
		"s3_endpoint":          "omitempty",
		"s3_access_key_id":     "omitempty",
		"s3_secret_access_key": "omitempty",
		"s3_bucket":            "omitempty",
		"s3_region":            "omitempty",
		"storage_config":       "omitempty",
	}).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Validate backup configuration before creation
		if err := backup.ValidateAutoBackupConfig(&ctx.Model); err != nil {
			ctx.AbortWithError(err)
			return
		}
		validatePluginStorage(ctx)
	}).ExecutedHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Register cron job only if the backup is enabled
		if ctx.Model.Enabled {
			if err := cron.AddAutoBackupJob(ctx.Model.ID, ctx.Model.CronExpression); err != nil {
				ctx.AbortWithError(err)
				return
			}
		}
	}).Create()
}

// GetAutoBackup retrieves a single auto backup configuration by ID.
//
// Path Parameters:
//   - id: Auto backup configuration ID
//
// Response: Auto backup configuration details
func GetAutoBackup(c *gin.Context) {
	core := cosy.Core[model.AutoBackup](c)
	if internalmcp.IsServiceTokenRequest(c) {
		core.SetTransformer(redactAutoBackup)
	}
	core.Get()
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
	cosy.Core[model.AutoBackup](c).SetValidRules(gin.H{
		"name":                 "omitempty",
		"backup_type":          "omitempty",
		"storage_type":         "omitempty",
		"storage_path":         "omitempty",
		"cron_expression":      "omitempty",
		"backup_path":          "omitempty",
		"enabled":              "omitempty",
		"retention_count":      "omitempty,min=0",
		"s3_endpoint":          "omitempty",
		"s3_access_key_id":     "omitempty",
		"s3_secret_access_key": "omitempty",
		"s3_bucket":            "omitempty",
		"s3_region":            "omitempty",
		"storage_config":       "omitempty",
	}).BeforeExecuteHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Validate backup configuration before modification
		if err := backup.ValidateAutoBackupUpdate(&ctx.Model, &ctx.OriginModel); err != nil {
			ctx.AbortWithError(err)
			return
		}
		validatePluginStorage(ctx)
	}).ExecutedHook(func(ctx *cosy.Ctx[model.AutoBackup]) {
		// Update cron job based on enabled status
		if ctx.Model.Enabled {
			if err := cron.UpdateAutoBackupJob(ctx.Model.ID, ctx.Model.CronExpression); err != nil {
				ctx.AbortWithError(err)
				return
			}
		} else {
			if err := cron.RemoveAutoBackupJob(ctx.Model.ID); err != nil {
				ctx.AbortWithError(err)
				return
			}
		}
	}).Modify()
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

// RunAutoBackup immediately executes a saved auto backup configuration.
// The enabled flag only controls cron scheduling and does not prevent a manual run.
func RunAutoBackup(c *gin.Context) {
	autoBackup, err := backup.GetAutoBackupByID(cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err := backup.ExecuteAutoBackup(autoBackup); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Auto backup completed successfully"})
}

// RestoreAutoBackup recovers a soft-deleted auto backup configuration and
// re-registers its cron job when it is enabled.
//
// Path Parameters:
//   - id: Auto backup configuration ID to recover
//
// Response: 204 No Content
func RestoreAutoBackup(c *gin.Context) {
	core := cosy.Core[model.AutoBackup](c)
	core.Recover()

	// Cosy's Recover chain does not run the executed hook, so the job is
	// registered here once the recover has answered 204.
	if c.Writer.Status() != http.StatusNoContent || !core.Model.Enabled {
		return
	}
	if err := cron.AddAutoBackupJob(core.Model.ID, core.Model.CronExpression); err != nil {
		logger.Errorf("Failed to add auto backup job %d after restore: %v", core.Model.ID, err)
	}
}
