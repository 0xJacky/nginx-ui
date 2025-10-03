package version

import "github.com/uozi-tech/cosy"

var (
	e                   = cosy.NewErrorScope("version")
	ErrInvalidCommitSHA = e.New(53001, "invalid commit SHA")
	ErrReleaseAPIFailed = e.New(53002, "release API request failed: {0}")
)
