package plugin

import (
	"context"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
)

const (
	// maintenanceInterval is how often the catalog is refreshed and the
	// automatic updates are applied.
	maintenanceInterval = 24 * time.Hour
	// maintenanceDelay keeps the first catalog refresh out of the boot path.
	maintenanceDelay = 5 * time.Minute
)

// marketplaces holds one client per manager. The manager struct itself lives
// in manager.go, which the marketplace feature does not own.
var (
	marketplaceMu sync.Mutex
	marketplaces  = map[*Manager]*Marketplace{}
)

// Marketplace returns the catalog client of this manager, creating it on the
// first call.
func (m *Manager) Marketplace() *Marketplace {
	marketplaceMu.Lock()
	defer marketplaceMu.Unlock()

	if existing, ok := marketplaces[m]; ok {
		return existing
	}
	created := newMarketplace(m)
	marketplaces[m] = created
	return created
}

// StartMarketplace runs the marketplace side of the boot sequence: it refreshes
// the partner keyring, installs whatever was dropped into the offline package
// directory, schedules the daily catalog maintenance and makes sure the DNS-01
// plugin is there when a certificate needs it. It returns immediately, the
// work happens in the background.
func (m *Manager) StartMarketplace(ctx context.Context) {
	if !settings.PluginSettings.Enabled {
		return
	}

	m.scheduleMaintenance()

	go m.refreshPartners(ctx)

	go func() {
		if err := m.ScanLocalPackages(ctx); err != nil {
			m.log.Warnf("Scan local plugin packages: %v", err)
		}
		m.EnsureDNS01Plugin(ctx)
	}()
}

// scheduleMaintenance registers the daily catalog job on the scheduler the
// manager already owns.
func (m *Manager) scheduleMaintenance() {
	m.mu.RLock()
	scheduler := m.scheduler
	m.mu.RUnlock()
	if scheduler == nil {
		// Offline mode and the command line tools have no scheduler.
		return
	}

	_, err := scheduler.NewJob(
		gocron.DurationJob(maintenanceInterval),
		gocron.NewTask(func() { m.runMaintenance() }),
		gocron.WithName("plugin_marketplace_maintenance"),
		gocron.WithStartAt(gocron.WithStartDateTime(time.Now().Add(maintenanceDelay))),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		m.log.Errorf("Schedule plugin marketplace maintenance: %v", err)
	}
}

// runMaintenance refreshes the catalog, reports updates and re-checks the
// DNS-01 plugin.
func (m *Manager) runMaintenance() {
	ctx := m.context()
	m.Marketplace().runMaintenance(ctx)
	m.EnsureDNS01Plugin(ctx)
}

// approvedPermissions returns the permission set the user approved for an
// installed plugin, nil when it was never approved.
func (m *Manager) approvedPermissions(id string) ([]string, bool) {
	item, ok := m.lookup(id)
	if !ok {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if item.row == nil {
		return nil, false
	}
	return item.row.ApprovedPermissions, true
}
