package docker

import "testing"

func TestCombineExecOutputPreservesSuccessfulStderr(t *testing.T) {
	stdout := "configuration output\n"
	stderr := "nginx: [warn] conflicting server name\n"

	if got := combineExecOutput(stdout, stderr); got != stdout+stderr {
		t.Fatalf("combineExecOutput() = %q, want %q", got, stdout+stderr)
	}
}
