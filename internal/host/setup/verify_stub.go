//go:build !linux

package setup

import (
	"context"
	"errors"
	"strings"
)

// StepOutcome is a single check result. Detail is raw evidence, while
// Remediation is a language-neutral message contract rendered by the UI.
type StepOutcome struct {
	OK          bool             `json:"ok"`
	Level       string           `json:"level,omitempty"`
	Detail      string           `json:"detail"`
	Remediation *StepRemediation `json:"remediation,omitempty"`
}

// VerifyResult aggregates all step outcomes.
type VerifyResult struct {
	Steps map[string]StepOutcome `json:"steps"`
}

// VerifyOptions narrows the input to what verify actually needs.
type VerifyOptions struct {
	Client     CommandRunner
	Params     SetupParams
	SkipNginxT bool
	// Groups limits the pipeline to the listed check groups. Empty runs all.
	Groups []CheckGroup
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
