package analytic

import "github.com/uozi-tech/cosy"

var (
	e                      = cosy.NewErrorScope("analytic")
	ErrNodeAnalyticsFailed = e.New(54001, "node analytics failed: {0}")
)
