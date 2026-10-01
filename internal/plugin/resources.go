package plugin

import (
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
)

// ResourceLimits are the limits of one plugin process, 0 meaning unlimited.
type ResourceLimits struct {
	// MemoryMB is memory in MiB.
	MemoryMB int
	// CPUPercent is CPU time in percent of one core, 100 being one core.
	CPUPercent int
}

// IsZero reports whether no limit applies.
func (l ResourceLimits) IsZero() bool {
	return l.MemoryMB <= 0 && l.CPUPercent <= 0
}

// ResourceStatus reports the limits of a plugin process in the plugin info.
type ResourceStatus struct {
	// MemoryLimitMB is the memory limit in MiB, 0 when unlimited.
	MemoryLimitMB int `json:"memory_limit_mb"`
	// CPUPercent is the CPU limit in percent of one core, 0 when unlimited.
	CPUPercent int `json:"cpu_percent"`
	// Enforced is true while the running process is confined to the limits.
	Enforced bool `json:"enforced"`
}

// EffectiveResources caps the server.resources hints of a manifest by the
// host limits. For each resource the smaller of the two wins, 0 being
// unlimited, so a hint lowers a limit but never raises it.
func EffectiveResources(host ResourceLimits, manifest *protocol.Manifest) ResourceLimits {
	var hint protocol.ManifestResources
	if manifest != nil && manifest.Server != nil && manifest.Server.Resources != nil {
		hint = *manifest.Server.Resources
	}
	return ResourceLimits{
		MemoryMB:   smallerLimit(host.MemoryMB, hint.MemoryMB),
		CPUPercent: smallerLimit(host.CPUPercent, hint.CPUPercent),
	}
}

// smallerLimit returns the smaller of two limits where a value below 1 means
// unlimited.
func smallerLimit(a, b int) int {
	switch {
	case a <= 0:
		return max(b, 0)
	case b <= 0:
		return a
	default:
		return min(a, b)
	}
}

// hostResourceLimits reads the limits of the plugin settings.
func hostResourceLimits() ResourceLimits {
	return ResourceLimits{
		MemoryMB:   settings.PluginSettings.MemoryLimitMB,
		CPUPercent: settings.PluginSettings.CPUPercent,
	}
}
