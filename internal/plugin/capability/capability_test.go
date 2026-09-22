package capability

import (
	"context"
	"errors"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
)

// capabilityHost is a NotifyHost, ProbeHost and MCPHost whose answers the
// test controls.
type capabilityHost struct {
	mu         sync.Mutex
	owners     map[string]map[string]string
	callers    map[string]*fakeCaller
	acquireErr error
	acquired   []string
	releases   int

	channels []plugin.NotifyChannelEntry
	kinds    []plugin.ProbeKindEntry
	tools    []plugin.MCPToolEntry
}

func newCapabilityHost() *capabilityHost {
	return &capabilityHost{owners: map[string]map[string]string{}, callers: map[string]*fakeCaller{}}
}

func (h *capabilityHost) own(capability, code, pluginID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.owners[capability] == nil {
		h.owners[capability] = map[string]string{}
	}
	h.owners[capability][code] = pluginID
}

func (h *capabilityHost) OwnerOf(capability, code string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	owner, ok := h.owners[capability][code]
	return owner, ok
}

func (h *capabilityHost) Acquire(_ context.Context, pluginID string) (jsonrpc.Caller, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.acquired = append(h.acquired, pluginID)
	release := func() {
		h.mu.Lock()
		h.releases++
		h.mu.Unlock()
	}
	if h.acquireErr != nil {
		return nil, release, h.acquireErr
	}
	caller, ok := h.callers[pluginID]
	if !ok {
		return nil, release, errors.New("plugin is not running")
	}
	return caller, release, nil
}

func (h *capabilityHost) NotifyChannels() []plugin.NotifyChannelEntry { return h.channels }
func (h *capabilityHost) ProbeKinds() []plugin.ProbeKindEntry         { return h.kinds }

func (h *capabilityHost) MCPTools() []plugin.MCPToolEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]plugin.MCPToolEntry(nil), h.tools...)
}

func (h *capabilityHost) setTools(tools []plugin.MCPToolEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tools = tools
}

func (h *capabilityHost) releaseCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.releases
}
