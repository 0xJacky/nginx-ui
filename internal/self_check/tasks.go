package self_check

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/uozi-tech/cosy"
)

type Task struct {
	Name      string
	CheckFunc func() error
	FixFunc   func() error
}

type Report struct {
	Name string      `json:"name"`
	Err  *cosy.Error `json:"err,omitempty"`
}

type Reports []*Report

var selfCheckTasks = []*Task{
	{
		Name:      "Directory-Sites",
		CheckFunc: CheckSitesDirectory,
		FixFunc:   FixSitesDirectory,
	},
	{
		Name:      "Directory-Streams",
		CheckFunc: CheckStreamDirectory,
		FixFunc:   FixStreamDirectory,
	},
	{
		Name:      "NginxConf-Sites-Enabled",
		CheckFunc: CheckNginxConfIncludeSites,
		FixFunc:   FixNginxConfIncludeSites,
	},
	{
		Name:      "NginxConf-Streams-Enabled",
		CheckFunc: CheckNginxConfIncludeStreams,
		FixFunc:   FixNginxConfIncludeStreams,
	},
	{
		Name:      "NginxConf-ConfD",
		CheckFunc: CheckNginxConfIncludeConfD,
		FixFunc:   FixNginxConfIncludeConfD,
	},
}

var selfCheckTaskMap = make(map[string]*Task)

func init() {
	for _, task := range selfCheckTasks {
		selfCheckTaskMap[task.Name] = task
	}
	if helper.InNginxUIOfficialDocker() {
		selfCheckTasks = append(selfCheckTasks, &Task{
			Name:      "Docker-Socket",
			CheckFunc: CheckDockerSocket,
		})
	}
}
