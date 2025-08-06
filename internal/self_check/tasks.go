package self_check

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/elliotchance/orderedmap/v3"
	"github.com/uozi-tech/cosy"
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
		Key:         "Directory-ConfD",
		Name:        translation.C("Conf.d directory exists"),
		Description: translation.C("Check if the conf.d directory is under the nginx configuration directory"),
		CheckFunc:   CheckConfDirectory,
		FixFunc:     FixConfDirectory,
	},
	{
		Key:  "NginxConf-ConfD-Include",
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
		Key:  "NginxPID-Path",
		Name: translation.C("Nginx PID path exists"),
		Description: translation.C("Check if the nginx PID path exists. " +
			"By default, this path is obtained from 'nginx -V'. If it cannot be obtained, an error will be reported. " +
			"In this case, you need to modify the configuration file to specify the Nginx PID path." +
			"Refer to the docs for more details: https://nginxui.com/zh_CN/guide/config-nginx.html#pidpath"),
		CheckFunc: CheckPIDPath,
	},
	{
		Key:         "NginxSbin-Path",
		Name:        translation.C("Nginx sbin path exists"),
		Description: translation.C("Check if the nginx sbin path exists"),
		CheckFunc:   CheckSbinPath,
	},
	{
		Key:  "NginxAccessLog-Path",
		Name: translation.C("Nginx access log path exists"),
		Description: translation.C("Check if the nginx access log path exists. " +
			"By default, this path is obtained from 'nginx -V'. If it cannot be obtained or the obtained path does not point to a valid, " +
			"existing file, an error will be reported. In this case, you need to modify the configuration file to specify the access log path." +
			"Refer to the docs for more details: https://nginxui.com/zh_CN/guide/config-nginx.html#accesslogpath"),
		CheckFunc: CheckAccessLogPath,
	},
	{
		Key:  "NginxErrorLog-Path",
		Name: translation.C("Nginx error log path exists"),
		Description: translation.C("Check if the nginx error log path exists. " +
			"By default, this path is obtained from 'nginx -V'. If it cannot be obtained or the obtained path does not point to a valid, " +
			"existing file, an error will be reported. In this case, you need to modify the configuration file to specify the error log path. " +
			"Refer to the docs for more details: https://nginxui.com/zh_CN/guide/config-nginx.html#errorlogpath"),
		CheckFunc: CheckErrorLogPath,
	},
}

var selfCheckTaskMap = orderedmap.NewOrderedMap[string, *Task]()

func Init() {
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
			Name: translation.C("Docker socket exists"),
			Description: translation.C("Check if /var/run/docker.sock exists. " +
				"If you are using Nginx UI Official " +
				"Docker Image, please make sure the docker socket is mounted like this: `-" +
				"v /var/run/docker.sock:/var/run/docker.sock`. " +
				"Nginx UI official image uses /var/run/docker.sock to communicate with the host Docker Engine via Docker Client API. " +
				"This feature is used to control Nginx in another container and perform container replacement rather than binary replacement " +
				"during OTA upgrades of Nginx UI to ensure container dependencies are also upgraded. " +
				"If you don't need this feature, please add the environment variable NGINX_UI_IGNORE_DOCKER_SOCKET=true to the container."),
			CheckFunc: CheckDockerSocket,
		})
	}

	for _, task := range selfCheckTasks {
		selfCheckTaskMap.Set(task.Key, task)
	}
}
