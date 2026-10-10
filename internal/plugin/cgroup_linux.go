//go:build linux

package plugin

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// startProcess starts cmd inside the cgroup of the plugin when limits apply
// and cgroup v2 can be written, and unconfined otherwise. It returns the
// command that actually runs, which is a fresh copy of cmd when the first
// start failed.
func (s *Supervisor) startProcess(cmd *exec.Cmd) (*exec.Cmd, *pluginCgroup, error) {
	limits := s.cfg.Resources
	if limits.IsZero() {
		return cmd, nil, cmd.Start()
	}

	cg, err := createCgroup(s.cgroupRoot(), s.cfg.PluginID, limits)
	if err != nil {
		if errors.Is(err, errCgroupV2Missing) {
			s.log.Debugf("[plugin:%s] resource limits need cgroup v2, running without them: %v", s.cfg.PluginID, err)
		} else {
			warnCgroupOnce(s.log, s.cfg.PluginID, err)
		}
		return cmd, nil, cmd.Start()
	}

	// clone3 starts the child inside the group, so it never runs outside.
	if dir, openErr := os.Open(cg.dir); openErr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: int(dir.Fd())}
		err = cmd.Start()
		_ = dir.Close()
		if err == nil {
			return cmd, cg, nil
		}
		// Linux before 5.7 has no CLONE_INTO_CGROUP: start the process
		// outside and move it in right away.
		s.log.Debugf("[plugin:%s] starting inside the cgroup failed, moving the process instead: %v", s.cfg.PluginID, err)
		cmd = cloneCommand(cmd)
	}

	if err = cmd.Start(); err != nil {
		_ = cg.remove()
		return cmd, nil, err
	}
	if err = cg.adopt(cmd.Process.Pid); err != nil {
		warnCgroupOnce(s.log, s.cfg.PluginID, err)
		_ = cg.remove()
		return cmd, nil, nil
	}
	return cmd, cg, nil
}

// cloneCommand copies a command that failed to start so it can be started
// again.
func cloneCommand(cmd *exec.Cmd) *exec.Cmd {
	return &exec.Cmd{
		Path:   cmd.Path,
		Args:   cmd.Args,
		Env:    cmd.Env,
		Dir:    cmd.Dir,
		Stdin:  cmd.Stdin,
		Stdout: cmd.Stdout,
		Stderr: cmd.Stderr,
	}
}
