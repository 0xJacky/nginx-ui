package self_check

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/elliotchance/orderedmap/v3"
	"github.com/uozi-tech/cosy"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

type Task struct {
	Key         string
	Name        *translation.Container
	Description *translation.Container
	CheckFunc   func() error
	FixFunc     func() error
}

type ReportStatus string

const (
	ReportStatusSuccess ReportStatus = "success"
	ReportStatusWarning ReportStatus = "warning"
	ReportStatusError   ReportStatus = "error"
)

type Report struct {
	Key         string                 `json:"key"`
	Name        *translation.Container `json:"name"`
	Description *translation.Container `json:"description,omitempty"`
	Fixable     bool                   `json:"fixable"`
	Err         *cosy.Error            `json:"err,omitempty"`
	Status      ReportStatus           `json:"status"`
}

type Reports []*Report

var selfCheckTasks = []*Task{
	{
		Key:  "Directory-Sites",
		Name: translation.C("Sites directory exists"),
		Description: translation.C("Check if the " +
			"sites-available and sites-enabled directories are " +
			"under the nginx configuration directory"),
		CheckFunc: CheckSitesDirectory,
		FixFunc:   FixSitesDirectory,
	},
	{
		Key:  "NginxConf-Sites-Enabled",
		Name: translation.C("Nginx.conf includes sites-enabled directory"),
		Description: translation.C("Check if the nginx.conf includes the " +
			"sites-enabled directory"),
		CheckFunc: CheckNginxConfIncludeSites,
		FixFunc:   FixNginxConfIncludeSites,
	},
	{
		Key:  "NginxConf-ConfD",
		Name: translation.C("Nginx.conf includes conf.d directory"),
		Description: translation.C("Check if the nginx.conf includes the " +
			"conf.d directory"),
		CheckFunc: CheckNginxConfIncludeConfD,
		FixFunc:   FixNginxConfIncludeConfD,
	},
	{
		Key:         "NginxConf-Directory",
		Name:        translation.C("Nginx configuration directory exists"),
		Description: translation.C("Check if the nginx configuration directory exists"),
		CheckFunc:   CheckConfigDir,
	},
	{
		Key:         "NginxConf-Entry-File",
		Name:        translation.C("Nginx configuration entry file exists"),
		Description: translation.C("Check if the nginx configuration entry file exists"),
		CheckFunc:   CheckConfigEntryFile,
	},
	{
		Key:         "NginxPID-Path",
		Name:        translation.C("Nginx PID path exists"),
		Description: translation.C("Check if the nginx PID path exists"),
		CheckFunc:   CheckPIDPath,
	},
	{
		Key:         "NginxAccessLog-Path",
		Name:        translation.C("Nginx access log path exists"),
		Description: translation.C("Check if the nginx access log path exists"),
		CheckFunc:   CheckAccessLogPath,
	},
	{
		Key:         "NginxErrorLog-Path",
		Name:        translation.C("Nginx error log path exists"),
		Description: translation.C("Check if the nginx error log path exists"),
		CheckFunc:   CheckErrorLogPath,
	},
}

var selfCheckTaskMap = orderedmap.NewOrderedMap[string, *Task]()

func init() {
	if nginx.IsModuleLoaded(nginx.ModuleStream) {
		selfCheckTasks = append(selfCheckTasks, &Task{
			Key:  "Directory-Streams",
			Name: translation.C("Streams directory exists"),
			Description: translation.C("Check if the " +
				"streams-available and streams-enabled directories are " +
				"under the nginx configuration directory"),
			CheckFunc: CheckStreamDirectory,
			FixFunc:   FixStreamDirectory,
		}, &Task{
			Key:  "NginxConf-Streams-Enabled",
			Name: translation.C("Nginx.conf includes streams-enabled directory"),
			Description: translation.C("Check if the nginx.conf includes the " +
			"streams-enabled directory"),
			CheckFunc: CheckNginxConfIncludeStreams,
			FixFunc:   FixNginxConfIncludeStreams,
		})
	}
	if helper.InNginxUIOfficialDocker() {
		selfCheckTasks = append(selfCheckTasks, &Task{
			Name:        translation.C("Docker socket exists"),
			Description: translation.C("Check if /var/run/docker.sock exists. If you are using Nginx UI Official " +
				"Docker Image, please make sure the docker socket is mounted like this: `-" +
				"v /var/run/docker.sock:/var/run/docker.sock`."),
			CheckFunc: CheckDockerSocket,
		})
	}

	for _, task := range selfCheckTasks {
		selfCheckTaskMap.Set(task.Key, task)
	}
}
