//go:build !linux

package plugin

import (
	"os/exec"
	"sync/atomic"
)

// cgroupNoticed keeps the "not enforced here" note to one per host process.
var cgroupNoticed atomic.Bool

// startProcess starts cmd without limits: they are enforced on Linux with
// cgroup v2 only (spec LIFE-16).
func (s *Supervisor) startProcess(cmd *exec.Cmd) (*exec.Cmd, *pluginCgroup, error) {
	if !s.cfg.Resources.IsZero() && cgroupNoticed.CompareAndSwap(false, true) {
		s.log.Debugf("[plugin:%s] resource limits of plugin processes are enforced on Linux with cgroup v2 only", s.cfg.PluginID)
	}
	return cmd, nil, cmd.Start()
}
