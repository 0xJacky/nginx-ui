//go:build !linux

package setup

import (
	"context"
	"errors"
)

// Verify stub for non-Linux build targets; SSH host mode is Linux-only by design.
func Verify(_ context.Context, _ VerifyOptions) VerifyResult {
	return VerifyResult{Steps: map[string]StepOutcome{
		"platform": {OK: false, Detail: errors.New("host_via_ssh verify is only supported on Linux containers").Error()},
	}}
}
