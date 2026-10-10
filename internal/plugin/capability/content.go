package capability

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/uozi-tech/cosy/logger"
)

// ContentHost is the part of the plugin manager content plugins need.
type ContentHost interface {
	// ContentEntries lists the content of every enabled plugin.
	ContentEntries() []plugin.ContentEntry
}

// RegisterContent adds the templates of every enabled plugin to the
// template lists and merges their translation files into the catalogs the
// host serves, keeping both in step with the plugin inventory.
func RegisterContent(h ContentHost) {
	template.RegisterSource(NewTemplateSource(h))

	bridge := NewLocaleBridge(h)
	bridge.Sync()
	event.Subscribe(func(published event.Event) {
		if published.Type != event.TypePluginChanged {
			return
		}
		// Bus subscribers must not block.
		go bridge.Sync()
	})
}

// NewTemplateSource exposes the templates directories of the plugins of h.
// They are read on every request, so a disabled plugin disappears at once.
func NewTemplateSource(h ContentHost) template.Source {
	return templateSource{host: h}
}

type templateSource struct {
	host ContentHost
}

func (s templateSource) TemplateRoots() []template.Root {
	var roots []template.Root
	for _, entry := range s.host.ContentEntries() {
		if entry.TemplatesDir != "" {
			roots = append(roots, template.Root{PluginID: entry.PluginID, Dir: entry.TemplatesDir})
		}
	}
	return roots
}

// LocaleBridge loads the translation files of the enabled plugins into the
// translation catalogs.
type LocaleBridge struct {
	host ContentHost
	// mu keeps syncs in order, so an older inventory never wins.
	mu sync.Mutex
}

// NewLocaleBridge returns a bridge that has loaded nothing yet.
func NewLocaleBridge(h ContentHost) *LocaleBridge {
	return &LocaleBridge{host: h}
}

// Sync replaces the plugin catalogs with the translation files of the
// plugins enabled right now. A file that does not parse is skipped.
func (b *LocaleBridge) Sync() {
	b.mu.Lock()
	defer b.mu.Unlock()

	catalogs := map[string]translation.Catalog{}
	for _, entry := range b.host.ContentEntries() {
		if entry.LocalesDir == "" {
			continue
		}
		catalog, errs := plugin.LoadLocales(entry.LocalesDir)
		for _, err := range errs {
			logger.Warnf("[plugin:%s] skip translation file %v", entry.PluginID, err)
		}
		if len(catalog) > 0 {
			catalogs[entry.PluginID] = catalog
		}
	}
	translation.SetPluginCatalogs(catalogs)
}
