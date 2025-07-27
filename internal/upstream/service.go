package upstream

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/uozi-tech/cosy/logger"
)

// TargetInfo contains proxy target information with source config
type TargetInfo struct {
	ProxyTarget
	ConfigPath string    `json:"config_path"`
	LastSeen   time.Time `json:"last_seen"`
}

// UpstreamDefinition contains upstream block information
type UpstreamDefinition struct {
	Name       string        `json:"name"`
	Servers    []ProxyTarget `json:"servers"`
	ConfigPath string        `json:"config_path"`
	LastSeen   time.Time     `json:"last_seen"`
}

// UpstreamService manages upstream availability testing
type UpstreamService struct {
	targets         map[string]*TargetInfo // key: host:port
	availabilityMap map[string]*Status     // key: host:port
	configTargets   map[string][]string    // configPath -> []targetKeys
	// Public upstream definitions storage
	Upstreams      map[string]*UpstreamDefinition // key: upstream name
	upstreamsMutex sync.RWMutex
	targetsMutex   sync.RWMutex
	lastUpdateTime time.Time
	testInProgress bool
	testMutex      sync.Mutex
}

var (
	upstreamService *UpstreamService
	serviceOnce     sync.Once
)

// GetUpstreamService returns the singleton upstream service instance
func GetUpstreamService() *UpstreamService {
	serviceOnce.Do(func() {
		upstreamService = &UpstreamService{
			targets:         make(map[string]*TargetInfo),
			availabilityMap: make(map[string]*Status),
			configTargets:   make(map[string][]string),
			Upstreams:       make(map[string]*UpstreamDefinition),
			lastUpdateTime:  time.Now(),
		}
	})
	return upstreamService
}

// init registers the ParseProxyTargetsFromRawContent callback
func init() {
	cache.RegisterCallback(scanForProxyTargets)
}

// scanForProxyTargets is the callback function for cache scanner
func scanForProxyTargets(configPath string, content []byte) error {
	logger.Debug("scanForProxyTargets", configPath)
	// Parse proxy targets and upstream definitions from config content
	result := ParseProxyTargetsAndUpstreamsFromRawContent(string(content))

	service := GetUpstreamService()
	service.updateTargetsFromConfig(configPath, result.ProxyTargets)

	// Update upstream definitions
	for upstreamName, servers := range result.Upstreams {
		service.UpdateUpstreamDefinition(upstreamName, servers, configPath)
	}

	return nil
}

// updateTargetsFromConfig updates proxy targets from a specific config file
func (s *UpstreamService) updateTargetsFromConfig(configPath string, targets []ProxyTarget) {
	s.targetsMutex.Lock()
	defer s.targetsMutex.Unlock()

	now := time.Now()

	// Remove old targets from this config path
	if oldTargetKeys, exists := s.configTargets[configPath]; exists {
		for _, key := range oldTargetKeys {
			if _, exists := s.targets[key]; exists {
				// Only remove if this is the only config using this target
				isOnlyConfig := true
				for otherConfig, otherKeys := range s.configTargets {
					if otherConfig != configPath {
						if slices.Contains(otherKeys, key) {
							isOnlyConfig = false
						}
						if !isOnlyConfig {
							break
						}
					}
				}
				if isOnlyConfig {
					delete(s.targets, key)
					delete(s.availabilityMap, key)
					// logger.Debug("Removed proxy target:", key, "from config:", configPath)
				} else {
					// logger.Debug("Keeping proxy target:", key, "still used by other configs")
				}
			}
		}
	}

	// Add/update new targets
	newTargetKeys := make([]string, 0, len(targets))
	for _, target := range targets {
		key := target.Host + ":" + target.Port
		newTargetKeys = append(newTargetKeys, key)

		if existingTarget, exists := s.targets[key]; exists {
			// Update existing target with latest info
			existingTarget.LastSeen = now
			existingTarget.ConfigPath = configPath // Update to latest config that referenced it
			// logger.Debug("Updated proxy target:", key, "from config:", configPath)
		} else {
			// Add new target
			s.targets[key] = &TargetInfo{
				ProxyTarget: target,
				ConfigPath:  configPath,
				LastSeen:    now,
			}
			// logger.Debug("Added proxy target:", key, "type:", target.Type, "from config:", configPath)
		}
	}

	// Update config target mapping
	s.configTargets[configPath] = newTargetKeys
	s.lastUpdateTime = now

	// logger.Debug("Config", configPath, "updated with", len(targets), "targets")
}

// GetTargets returns a copy of current proxy targets
func (s *UpstreamService) GetTargets() []ProxyTarget {
	s.targetsMutex.RLock()
	defer s.targetsMutex.RUnlock()

	targets := make([]ProxyTarget, 0, len(s.targets))
	for _, targetInfo := range s.targets {
		targets = append(targets, targetInfo.ProxyTarget)
	}
	return targets
}

// GetTargetInfos returns a copy of current target infos
func (s *UpstreamService) GetTargetInfos() []*TargetInfo {
	s.targetsMutex.RLock()
	defer s.targetsMutex.RUnlock()

	targetInfos := make([]*TargetInfo, 0, len(s.targets))
	for _, targetInfo := range s.targets {
		// Create a copy
		targetInfoCopy := &TargetInfo{
			ProxyTarget: targetInfo.ProxyTarget,
			ConfigPath:  targetInfo.ConfigPath,
			LastSeen:    targetInfo.LastSeen,
		}
		targetInfos = append(targetInfos, targetInfoCopy)
	}
	return targetInfos
}

// GetAvailabilityMap returns a copy of current availability results
func (s *UpstreamService) GetAvailabilityMap() map[string]*Status {
	s.targetsMutex.RLock()
	defer s.targetsMutex.RUnlock()

	result := make(map[string]*Status)
	for k, v := range s.availabilityMap {
		// Create a copy of the status
		result[k] = &Status{
			Online:  v.Online,
			Latency: v.Latency,
		}
	}
	return result
}

// PerformAvailabilityTest performs availability test for all targets
func (s *UpstreamService) PerformAvailabilityTest() {
	// Prevent concurrent tests
	s.testMutex.Lock()
	if s.testInProgress {
		s.testMutex.Unlock()
		// logger.Debug("Availability test already in progress, skipping")
		return
	}
	s.testInProgress = true
	s.testMutex.Unlock()

	// Ensure we reset the flag when done
	defer func() {
		s.testMutex.Lock()
		s.testInProgress = false
		s.testMutex.Unlock()
	}()

	s.targetsMutex.RLock()
	targetCount := len(s.targets)
	s.targetsMutex.RUnlock()

	if targetCount == 0 {
		logger.Debug("No targets to test")
		return
	}

	logger.Debug("Performing availability test for", targetCount, "unique targets")

	// Separate targets into traditional and consul groups from the start
	s.targetsMutex.RLock()
	regularTargetKeys := make([]string, 0, len(s.targets))
	consulTargets := make([]ProxyTarget, 0, len(s.targets))

	for _, targetInfo := range s.targets {
		if targetInfo.ProxyTarget.IsConsul {
			consulTargets = append(consulTargets, targetInfo.ProxyTarget)
		} else {
			// Traditional target - use host:port key format
			key := targetInfo.ProxyTarget.Host + ":" + targetInfo.ProxyTarget.Port
			regularTargetKeys = append(regularTargetKeys, key)
		}
	}
	s.targetsMutex.RUnlock()

	// Initialize results map
	results := make(map[string]*Status)

	// Test traditional targets using the original AvailabilityTest
	if len(regularTargetKeys) > 0 {
		// logger.Debug("Testing", len(regularTargetKeys), "traditional targets")
		regularResults := AvailabilityTest(regularTargetKeys)
		maps.Copy(results, regularResults)
	}

	// Test consul targets using consul-specific logic
	if len(consulTargets) > 0 {
		// logger.Debug("Testing", len(consulTargets), "consul targets")
		consulResults := TestDynamicTargets(consulTargets)
		maps.Copy(results, consulResults)
	}

	// Update availability map
	s.targetsMutex.Lock()
	s.availabilityMap = results
	s.targetsMutex.Unlock()

	// logger.Debug("Availability test completed for", len(results), "targets")
}

// ClearTargets clears all targets (useful for testing or reloading)
func (s *UpstreamService) ClearTargets() {
	s.targetsMutex.Lock()
	s.upstreamsMutex.Lock()
	defer s.targetsMutex.Unlock()
	defer s.upstreamsMutex.Unlock()

	s.targets = make(map[string]*TargetInfo)
	s.availabilityMap = make(map[string]*Status)
	s.configTargets = make(map[string][]string)
	s.Upstreams = make(map[string]*UpstreamDefinition)
	s.lastUpdateTime = time.Now()

	// logger.Debug("Cleared all proxy targets and upstream definitions")
}

// GetLastUpdateTime returns the last time targets were updated
func (s *UpstreamService) GetLastUpdateTime() time.Time {
	s.targetsMutex.RLock()
	defer s.targetsMutex.RUnlock()
	return s.lastUpdateTime
}

// GetTargetCount returns the number of unique targets
func (s *UpstreamService) GetTargetCount() int {
	s.targetsMutex.RLock()
	defer s.targetsMutex.RUnlock()
	return len(s.targets)
}

// UpdateUpstreamDefinition updates or adds an upstream definition
func (s *UpstreamService) UpdateUpstreamDefinition(name string, servers []ProxyTarget, configPath string) {
	s.upstreamsMutex.Lock()
	defer s.upstreamsMutex.Unlock()

	s.Upstreams[name] = &UpstreamDefinition{
		Name:       name,
		Servers:    servers,
		ConfigPath: configPath,
		LastSeen:   time.Now(),
	}
}

// GetUpstreamDefinition returns an upstream definition by name
func (s *UpstreamService) GetUpstreamDefinition(name string) (*UpstreamDefinition, bool) {
	s.upstreamsMutex.RLock()
	defer s.upstreamsMutex.RUnlock()

	upstream, exists := s.Upstreams[name]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	return &UpstreamDefinition{
		Name:       upstream.Name,
		Servers:    append([]ProxyTarget(nil), upstream.Servers...),
		ConfigPath: upstream.ConfigPath,
		LastSeen:   upstream.LastSeen,
	}, true
}

// GetAllUpstreamDefinitions returns a copy of all upstream definitions
func (s *UpstreamService) GetAllUpstreamDefinitions() map[string]*UpstreamDefinition {
	s.upstreamsMutex.RLock()
	defer s.upstreamsMutex.RUnlock()

	result := make(map[string]*UpstreamDefinition)
	for name, upstream := range s.Upstreams {
		result[name] = &UpstreamDefinition{
			Name:       upstream.Name,
			Servers:    append([]ProxyTarget(nil), upstream.Servers...),
			ConfigPath: upstream.ConfigPath,
			LastSeen:   upstream.LastSeen,
		}
	}
	return result
}

// IsUpstreamName checks if a given name is a known upstream
func (s *UpstreamService) IsUpstreamName(name string) bool {
	s.upstreamsMutex.RLock()
	defer s.upstreamsMutex.RUnlock()
	_, exists := s.Upstreams[name]
	return exists
}

// RemoveConfigTargets removes all targets associated with a specific config file
func (s *UpstreamService) RemoveConfigTargets(configPath string) {
	s.targetsMutex.Lock()
	defer s.targetsMutex.Unlock()

	if targetKeys, exists := s.configTargets[configPath]; exists {
		for _, key := range targetKeys {
			// Check if this target is used by other configs
			isUsedByOthers := false
			for otherConfig, otherKeys := range s.configTargets {
				if otherConfig != configPath {
					if slices.Contains(otherKeys, key) {
						isUsedByOthers = true
					}
					if isUsedByOthers {
						break
					}
				}
			}

			if !isUsedByOthers {
				delete(s.targets, key)
				delete(s.availabilityMap, key)
				// logger.Debug("Removed proxy target:", key, "after config removal:", configPath)
			}
		}
		delete(s.configTargets, configPath)
		s.lastUpdateTime = time.Now()
		// logger.Debug("Removed config targets for:", configPath)
	}
}
