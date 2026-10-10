//go:build linux

package plugin

import (
	"os"
	"strconv"
)

// pluginOOMScoreAdj makes the kernel prefer a plugin over the host when
// memory runs out. Raising the score of a child needs no privilege.
const pluginOOMScoreAdj = 500

// preferOOMKill raises the OOM score of a plugin process. Failure is not
// important, the process just keeps the inherited score.
func (s *Supervisor) preferOOMKill(pid int) {
	if err := setOOMScoreAdj(pid, pluginOOMScoreAdj); err != nil {
		s.log.Debugf("[plugin:%s] set oom_score_adj: %v", s.cfg.PluginID, err)
	}
}

// setOOMScoreAdj writes the OOM score adjustment of a process.
func setOOMScoreAdj(pid, value int) error {
	path := "/proc/" + strconv.Itoa(pid) + "/oom_score_adj"
	return os.WriteFile(path, []byte(strconv.Itoa(value)), 0o644)
}
