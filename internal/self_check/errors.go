package self_check

import "github.com/uozi-tech/cosy"

var (
	e                                   = cosy.NewErrorScope("self_check")
	ErrTaskNotFound                     = e.New(40400, "Task not found")
	ErrTaskNotFixable                   = e.New(40401, "Task is not fixable")
	ErrFailedToReadNginxConf            = e.New(40402, "Failed to read nginx.conf")
	ErrParseNginxConf                   = e.New(50001, "Failed to parse nginx.conf")
	ErrNginxConfNoHttpBlock             = e.New(40403, "Nginx conf no http block")
	ErrNginxConfNotIncludeSitesEnabled  = e.New(40404, "Nginx conf not include sites-enabled")
	ErrNginxConfNoStreamBlock           = e.New(40405, "Nginx conf no stream block")
	ErrNginxConfNotIncludeStreamEnabled = e.New(40406, "Nginx conf not include stream-enabled")
	ErrFailedToCreateBackup             = e.New(50002, "Failed to create backup")
	ErrSitesAvailableNotExist           = e.New(40407, "Sites-available directory not exist")
	ErrSitesEnabledNotExist             = e.New(40408, "Sites-enabled directory not exist")
	ErrStreamAvailableNotExist          = e.New(40409, "Streams-available directory not exist")
	ErrStreamEnabledNotExist            = e.New(40410, "Streams-enabled directory not exist")
	ErrNginxConfNotIncludeConfD         = e.New(40411, "Nginx conf not include conf.d directory")
	ErrDockerSocketNotExist             = e.New(40412, "Docker socket not exist")
	ErrConfigDirNotExist                = e.New(40413, "Config directory not exist")
	ErrConfigEntryFileNotExist          = e.New(40414, "Config entry file not exist")
	ErrPIDPathNotExist                  = e.New(40415, "PID path not exist")
	ErrSbinPathNotExist                 = e.New(40416, "Sbin path not exist")
	ErrAccessLogPathNotExist            = e.New(40417, "Access log path not exist")
	ErrErrorLogPathNotExist             = e.New(40418, "Error log path not exist")
	ErrConfdNotExists                   = e.New(40419, "Conf.d directory not exists")
)
