package self_check

import (
	"errors"

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
}

var selfCheckTaskMap = make(map[string]*Task)

func init() {
	for _, task := range selfCheckTasks {
		selfCheckTaskMap[task.Name] = task
	}
}

func Run() (reports Reports) {
	reports = make(Reports, 0)
	for _, task := range selfCheckTasks {
		var cErr *cosy.Error
		if err := task.CheckFunc(); err != nil {
			errors.As(err, &cErr)
		}
		reports = append(reports, &Report{
			Name: task.Name,
			Err:  cErr,
		})
	}
	return
}

func AttemptFix(taskName string) (err error) {
	task, ok := selfCheckTaskMap[taskName]
	if !ok {
		return ErrTaskNotFound
	}
	return task.FixFunc()
}
