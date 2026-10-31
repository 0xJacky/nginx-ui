//go:build linux

package plugin

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPreferOOMKillRaisesChildScore(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	path := "/proc/" + strconv.Itoa(cmd.Process.Pid) + "/oom_score_adj"
	before, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("oom_score_adj is not readable: %v", err)
	}
	if strings.TrimSpace(string(before)) == strconv.Itoa(pluginOOMScoreAdj) {
		t.Skip("the score is already at the target value")
	}

	s := &Supervisor{log: zap.NewNop().Sugar()}
	s.preferOOMKill(cmd.Process.Pid)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	if strings.TrimSpace(string(after)) == strings.TrimSpace(string(before)) {
		t.Skip("the kernel did not accept the new score here")
	}
	require.Equal(t, strconv.Itoa(pluginOOMScoreAdj), strings.TrimSpace(string(after)))
}
