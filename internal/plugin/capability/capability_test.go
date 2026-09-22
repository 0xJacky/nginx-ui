package capability

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
)

// capabilityHost is a NotifyHost, ProbeHost, MCPHost, StorageHost and
// DeployHost whose answers the test controls.
type capabilityHost struct {
	mu         sync.Mutex
	owners     map[string]map[string]string
	callers    map[string]*fakeCaller
	handlers   map[string]jsonrpc.Caller
	acquireErr error
	acquired   []string
	releases   int

	channels  []plugin.NotifyChannelEntry
	kinds     []plugin.ProbeKindEntry
	tools     []plugin.MCPToolEntry
	backends  []plugin.StorageBackendEntry
	targets   []plugin.DeployTargetEntry
	sources   []plugin.BlocklistSourceEntry
	providers []plugin.DiscoveryProviderEntry
	content   []plugin.ContentEntry
	dataDirs  map[string]string
}

func newCapabilityHost() *capabilityHost {
	return &capabilityHost{
		owners:   map[string]map[string]string{},
		callers:  map[string]*fakeCaller{},
		handlers: map[string]jsonrpc.Caller{},
		dataDirs: map[string]string{},
	}
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
	if handler, ok := h.handlers[pluginID]; ok {
		return handler, release, nil
	}
	caller, ok := h.callers[pluginID]
	if !ok {
		return nil, release, errors.New("plugin is not running")
	}
	return caller, release, nil
}

func (h *capabilityHost) NotifyChannels() []plugin.NotifyChannelEntry   { return h.channels }
func (h *capabilityHost) ProbeKinds() []plugin.ProbeKindEntry           { return h.kinds }
func (h *capabilityHost) StorageBackends() []plugin.StorageBackendEntry { return h.backends }
func (h *capabilityHost) DeployTargets() []plugin.DeployTargetEntry     { return h.targets }
func (h *capabilityHost) DataDir(pluginID string) string                { return h.dataDirs[pluginID] }
func (h *capabilityHost) BlocklistSources() []plugin.BlocklistSourceEntry {
	return h.sources
}
func (h *capabilityHost) DiscoveryProviders() []plugin.DiscoveryProviderEntry {
	return h.providers
}

func (h *capabilityHost) ContentEntries() []plugin.ContentEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]plugin.ContentEntry(nil), h.content...)
}

func (h *capabilityHost) setContent(entries []plugin.ContentEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.content = entries
}

// handlerCaller runs a function for every call, so a test can act like the
// plugin, for example on the files of the exchange directory.
type handlerCaller func(method string, params json.RawMessage) (any, error)

func (f handlerCaller) Call(_ context.Context, method string, params any, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	answer, err := f(method, raw)
	if err != nil {
		return err
	}
	if result == nil || answer == nil {
		return nil
	}
	encoded, err := json.Marshal(answer)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, result)
}

func (handlerCaller) Notify(context.Context, string, any) error { return nil }

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
