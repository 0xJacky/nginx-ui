package self_check

import "github.com/uozi-tech/cosy"

var (
	e                                   = cosy.NewErrorScope("self_check")
	ErrTaskNotFound                     = e.New(4040, "Task not found")
	ErrFailedToReadNginxConf            = e.New(4041, "Failed to read nginx.conf")
	ErrParseNginxConf                   = e.New(5001, "Failed to parse nginx.conf")
	ErrNginxConfNoHttpBlock             = e.New(4042, "Nginx conf no http block")
	ErrNginxConfNotIncludeSitesEnabled  = e.New(4043, "Nginx conf not include sites-enabled")
	ErrorNginxConfNoStreamBlock         = e.New(4044, "Nginx conf no stream block")
	ErrNginxConfNotIncludeStreamEnabled = e.New(4045, "Nginx conf not include stream-enabled")
	ErrFailedToCreateBackup             = e.New(5001, "Failed to create backup")
	ErrSitesAvailableNotExist           = e.New(4046, "Sites-available directory not exist")
	ErrSitesEnabledNotExist             = e.New(4047, "Sites-enabled directory not exist")
	ErrStreamAvailableNotExist          = e.New(4048, "Stream-available directory not exist")
	ErrStreamEnabledNotExist            = e.New(4049, "Stream-enabled directory not exist")
)
