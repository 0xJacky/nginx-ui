//go:build !linux

package setup

import (
	"context"
	"errors"
	"strings"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
)

// StepOutcome is a single check result. Detail is the raw evidence;
// Remediation is a human-readable fix-it hint that the UI may render
// as a copy-pasteable shell command.
type StepOutcome struct {
	OK          bool   `json:"ok"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation,omitempty"`
}

// VerifyResult aggregates all step outcomes.
type VerifyResult struct {
	Steps map[string]StepOutcome `json:"steps"`
}

// VerifyOptions narrows the input to what verify actually needs.
type VerifyOptions struct {
	Client     *hostssh.Client
	Params     SetupParams
	SkipNginxT bool
}

// Verify stub for non-Linux build targets; SSH host mode is Linux-only by design.
func Verify(_ context.Context, _ VerifyOptions) VerifyResult {
	return VerifyResult{Steps: map[string]StepOutcome{
		"platform": {OK: false, Detail: errors.New("host_via_ssh verify is only supported on Linux containers").Error()},
	}}
}

// findMissingSudoEntries is testable on all platforms (no syscalls).
func findMissingSudoEntries(sudoListOutput string, required []string) []string {
	var missing []string
	for _, req := range required {
		if !strings.Contains(sudoListOutput, req) {
			missing = append(missing, req)
		}
	}
	return missing
}
