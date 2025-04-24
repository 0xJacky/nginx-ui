package self_check

import (
	"errors"

	"github.com/uozi-tech/cosy"
)



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
