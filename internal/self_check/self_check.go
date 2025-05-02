package self_check

import (
	"errors"

	"github.com/uozi-tech/cosy"
)

func Run() (reports Reports) {
	reports = make(Reports, 0)
	for _, task := range selfCheckTasks {
		var cErr *cosy.Error
		status := ReportStatusSuccess
		if err := task.CheckFunc(); err != nil {
			errors.As(err, &cErr)
			status = ReportStatusError
		}
		reports = append(reports, &Report{
			Key:         task.Key,
			Name:        task.Name,
			Description: task.Description,
			Fixable:     task.FixFunc != nil,
			Err:         cErr,
			Status:      status,
		})
	}
	return
}

func AttemptFix(taskName string) (err error) {
	task, ok := selfCheckTaskMap.Get(taskName)
	if !ok {
		return ErrTaskNotFound
	}
	if task.FixFunc == nil {
		return ErrTaskNotFixable
	}
	return task.FixFunc()
}
