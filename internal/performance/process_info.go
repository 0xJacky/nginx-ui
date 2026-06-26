package performance

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/shirou/gopsutil/v4/process"
)

type NginxProcessInfo struct {
	Workers     int     `json:"workers"`
	Master      int     `json:"master"`
	Cache       int     `json:"cache"`
	Other       int     `json:"other"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
}

// GetNginxProcessInfo Get Nginx process information.
// When nginx runs in a separate container its processes are not visible in the
// Nginx UI process namespace, so gopsutil would report zero workers (#1704).
// In that case the process list is collected from inside the nginx container.
func GetNginxProcessInfo() (*NginxProcessInfo, error) {
	if settings.NginxSettings.RunningInAnotherContainer() {
		return getNginxProcessInfoFromContainer()
	}
	return getNginxProcessInfoFromHost()
}

// getNginxProcessInfoFromHost counts nginx processes in the local process
// namespace (Nginx UI and nginx share the same host/container).
func getNginxProcessInfoFromHost() (*NginxProcessInfo, error) {
	result := &NginxProcessInfo{
		Workers:     0,
		Master:      0,
		Cache:       0,
		Other:       0,
		CPUUsage:    0.0,
		MemoryUsage: 0.0,
	}

	// Find all Nginx processes
	processes, err := process.Processes()
	if err != nil {
		return result, fmt.Errorf("failed to get processes: %v", err)
	}

	totalMemory := 0.0
	workerCount := 0
	masterCount := 0
	cacheCount := 0
	otherCount := 0
	nginxProcesses := make([]*process.Process, 0)

	// Get the number of system CPU cores
	numCPU := runtime.NumCPU()

	// Get the PID of the Nginx master process
	var masterPID int32 = -1
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}

		// Check if it is the Nginx master process
		if strings.Contains(strings.ToLower(name), "nginx") &&
			(strings.Contains(cmdline, "master process") ||
				!strings.Contains(cmdline, "worker process")) &&
			p.Pid > 0 {
			masterPID = p.Pid
			masterCount++
			nginxProcesses = append(nginxProcesses, p)

			// Get the memory usage
			mem, err := p.MemoryInfo()
			if err == nil && mem != nil {
				// Convert to MB
				memoryUsage := float64(mem.RSS) / 1024 / 1024
				totalMemory += memoryUsage
			}

			break
		}
	}

	// Iterate through all processes, distinguishing between worker processes and other Nginx processes
	for _, p := range processes {
		if p.Pid == masterPID {
			continue // Already calculated the master process
		}

		name, err := p.Name()
		if err != nil {
			continue
		}

		// Only process Nginx related processes
		if !strings.Contains(strings.ToLower(name), "nginx") {
			continue
		}

		// Add to the Nginx process list
		nginxProcesses = append(nginxProcesses, p)

		// Get the parent process PID
		ppid, err := p.Ppid()
		if err != nil {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}

		// Get the memory usage
		mem, err := p.MemoryInfo()
		if err == nil && mem != nil {
			// Convert to MB
			memoryUsage := float64(mem.RSS) / 1024 / 1024
			totalMemory += memoryUsage
		}

		// Distinguish between worker processes, cache processes, and other processes
		if ppid == masterPID && strings.Contains(cmdline, "worker process") {
			workerCount++
		} else if ppid == masterPID && strings.Contains(cmdline, "cache") {
			cacheCount++
		} else {
			otherCount++
		}
	}

	// Calculate the CPU usage
	// First, measure the initial CPU time
	times1 := make(map[int32]float64)
	for _, p := range nginxProcesses {
		times, err := p.Times()
		if err == nil {
			// CPU time = user time + system time
			times1[p.Pid] = times.User + times.System
		}
	}

	// Wait for a short period of time
	time.Sleep(100 * time.Millisecond)

	// Measure the CPU time again
	totalCPUPercent := 0.0
	for _, p := range nginxProcesses {
		times, err := p.Times()
		if err != nil {
			continue
		}

		// Calculate the CPU time difference
		currentTotal := times.User + times.System
		if previousTotal, ok := times1[p.Pid]; ok {
			// Calculate the CPU usage percentage during this period (considering multiple cores)
			cpuDelta := currentTotal - previousTotal
			// Calculate the CPU usage per second (considering the sampling time)
			cpuPercent := (cpuDelta / 0.1) * 100.0 / float64(numCPU)
			totalCPUPercent += cpuPercent
		}
	}

	// Round to the nearest integer, which is more consistent with the top display
	totalCPUPercent = math.Round(totalCPUPercent)

	// Round the memory usage to two decimal places
	totalMemory = math.Round(totalMemory*100) / 100

	result.Workers = workerCount
	result.Master = masterCount
	result.Cache = cacheCount
	result.Other = otherCount
	result.CPUUsage = totalCPUPercent
	result.MemoryUsage = totalMemory

	return result, nil
}

// getNginxProcessInfoFromContainer counts nginx processes by reading /proc
// inside the nginx container. It reads cmdlines directly instead of relying on
// `ps`, which is missing from slim/distroless nginx images. CPU and memory
// usage are left at zero here because gopsutil cannot sample a foreign
// container's processes; the master/worker/cache counts are what the dashboard
// reports as "worker processes".
func getNginxProcessInfoFromContainer() (*NginxProcessInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Emit one line per process: NUL-separated argv joined by spaces. nginx
	// retitles its processes (setproctitle), so master/worker/cache lines start
	// with "nginx: ... process".
	out, err := docker.Exec(ctx, []string{"sh", "-c",
		`for p in /proc/[0-9]*/cmdline; do tr '\0' ' ' < "$p" 2>/dev/null; echo; done`})
	if err != nil {
		return &NginxProcessInfo{}, fmt.Errorf("failed to inspect nginx container processes: %w", err)
	}

	return parseContainerNginxProcesses(out), nil
}

// parseContainerNginxProcesses counts nginx master/worker/cache processes from a
// newline-separated list of process command lines (NUL bytes already replaced
// by spaces). nginx retitles its processes via setproctitle, e.g.
// "nginx: master process ...", "nginx: worker process",
// "nginx: cache manager process".
func parseContainerNginxProcesses(procList string) *NginxProcessInfo {
	result := &NginxProcessInfo{}
	for _, line := range strings.Split(procList, "\n") {
		switch {
		case strings.Contains(line, "nginx: master process"):
			result.Master++
		case strings.Contains(line, "nginx: worker process"):
			result.Workers++
		case strings.Contains(line, "nginx: cache"):
			result.Cache++
		}
	}
	return result
}
