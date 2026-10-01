package plugin

// This file prepares the cgroup v2 group a plugin process is confined to.
// The group of a plugin is
// <root>/nginx-ui/plugins/<plugin id>: nginx-ui and nginx-ui/plugins only
// pass the controllers down, and the leaf holds the limits and the process.
// Starting the process inside it is Linux only, see cgroup_linux.go; the
// file handling here works on any directory, which is what the tests use.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	// cgroupBaseDir holds the groups of the plugins under the cgroup root.
	cgroupBaseDir = "nginx-ui"
	// cgroupPluginsDir is the parent of the group of every plugin.
	cgroupPluginsDir = "plugins"
	// cgroupCPUPeriod is the cpu.max period in microseconds. A CPU percent
	// of 100 is a quota of one period, one core.
	cgroupCPUPeriod = 100000
	// cgroupRemoveAttempts bounds how long removing a group waits for the
	// processes left in it to die.
	cgroupRemoveAttempts = 20
	cgroupRemoveDelay    = 25 * time.Millisecond
)

// errCgroupV2Missing reports a cgroup root without the unified hierarchy.
var errCgroupV2Missing = errors.New("no cgroup v2 hierarchy")

// cgroupWarned keeps the "cannot confine" warning to one per host process.
var cgroupWarned atomic.Bool

// warnCgroupOnce logs the first failure to confine a plugin at warning level
// and every later one at debug level.
func warnCgroupOnce(log *zap.SugaredLogger, pluginID string, err error) {
	if cgroupWarned.CompareAndSwap(false, true) {
		log.Warnf("[plugin:%s] plugin processes run without resource limits: %v", pluginID, err)
		return
	}
	log.Debugf("[plugin:%s] running without resource limits: %v", pluginID, err)
}

// pluginCgroup is the prepared group of one plugin process.
type pluginCgroup struct {
	dir    string
	limits ResourceLimits
}

// cgroupName turns a plugin id into one safe path segment.
func cgroupName(pluginID string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
			return r
		default:
			return '_'
		}
	}, pluginID)
	if name == "" || strings.Trim(name, ".") == "" {
		return "_" + name
	}
	return name
}

// cgroupDir is the group directory of a plugin under root.
func cgroupDir(root, pluginID string) string {
	return filepath.Join(root, cgroupBaseDir, cgroupPluginsDir, cgroupName(pluginID))
}

// createCgroup prepares the group of a plugin with the limits and returns
// it, or nil when no limit applies. It enables the controllers the limits
// need from the root down to the parent of the group.
func createCgroup(root, pluginID string, limits ResourceLimits) (*pluginCgroup, error) {
	if limits.IsZero() {
		return nil, nil
	}

	available, err := os.ReadFile(filepath.Join(root, "cgroup.controllers"))
	if err != nil {
		return nil, fmt.Errorf("%w at %s", errCgroupV2Missing, root)
	}
	var controllers []string
	if limits.MemoryMB > 0 {
		controllers = append(controllers, "memory")
	}
	if limits.CPUPercent > 0 {
		controllers = append(controllers, "cpu")
	}
	for _, controller := range controllers {
		if !slices.Contains(strings.Fields(string(available)), controller) {
			return nil, fmt.Errorf("the %s controller is not available at %s", controller, root)
		}
	}

	parents := []string{
		root,
		filepath.Join(root, cgroupBaseDir),
		filepath.Join(root, cgroupBaseDir, cgroupPluginsDir),
	}
	for i, dir := range parents {
		if i > 0 {
			if err = os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return nil, err
			}
		}
		if err = enableControllers(dir, controllers); err != nil {
			return nil, err
		}
	}

	dir := cgroupDir(root, pluginID)
	// A group left behind by a crash of the host is replaced; one that still
	// holds a process is reused.
	_ = os.RemoveAll(dir)
	if err = os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	cg := &pluginCgroup{dir: dir, limits: limits}
	if err = cg.writeLimits(); err != nil {
		_ = cg.remove()
		return nil, err
	}
	return cg, nil
}

// enableControllers turns on the controllers for the children of dir.
func enableControllers(dir string, controllers []string) error {
	path := filepath.Join(dir, "cgroup.subtree_control")
	current, _ := os.ReadFile(path)
	enabled := strings.Fields(string(current))

	var missing []string
	for _, controller := range controllers {
		if !slices.Contains(enabled, controller) {
			missing = append(missing, "+"+controller)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(missing, " ")), 0o644); err != nil {
		return fmt.Errorf("enable %s in %s: %w", strings.Join(missing, " "), dir, err)
	}
	return nil
}

// writeLimits writes the limits into the group.
func (c *pluginCgroup) writeLimits() error {
	if c.limits.MemoryMB > 0 {
		bytes := int64(c.limits.MemoryMB) << 20
		if err := c.write("memory.max", strconv.FormatInt(bytes, 10)); err != nil {
			return err
		}
		// A limited process must not swap its way around the limit. The
		// file is missing without swap accounting, which is fine.
		_ = c.write("memory.swap.max", "0")
	}
	if c.limits.CPUPercent > 0 {
		quota := c.limits.CPUPercent * cgroupCPUPeriod / 100
		if err := c.write("cpu.max", fmt.Sprintf("%d %d", quota, cgroupCPUPeriod)); err != nil {
			return err
		}
	}
	return nil
}

func (c *pluginCgroup) write(name, value string) error {
	if err := os.WriteFile(filepath.Join(c.dir, name), []byte(value), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// adopt moves a started process into the group, for kernels that cannot
// start it there directly.
func (c *pluginCgroup) adopt(pid int) error {
	return c.write("cgroup.procs", strconv.Itoa(pid))
}

// remove deletes the group once its process exited. Processes the plugin
// left behind are killed first.
func (c *pluginCgroup) remove() error {
	var err error
	for attempt := range cgroupRemoveAttempts {
		if err = os.RemoveAll(c.dir); err == nil {
			return nil
		}
		if attempt == 0 {
			// cgroup.kill exists from Linux 5.14 on.
			_ = os.WriteFile(filepath.Join(c.dir, "cgroup.kill"), []byte("1"), 0o644)
		}
		time.Sleep(cgroupRemoveDelay)
	}
	return err
}

// resourceStatus reports the limits and whether they are enforced.
func resourceStatus(limits ResourceLimits, enforced bool) ResourceStatus {
	return ResourceStatus{
		MemoryLimitMB: max(limits.MemoryMB, 0),
		CPUPercent:    max(limits.CPUPercent, 0),
		Enforced:      enforced,
	}
}
