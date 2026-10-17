package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cast"
)

// State of one plugin on one node, as the matrix reports it.
const (
	// SyncStateInSync means the node runs the same version in the same state.
	SyncStateInSync = "in_sync"
	// SyncStateOutdated means the node has the plugin but not as configured.
	SyncStateOutdated = "outdated"
	// SyncStateMissing means the node does not have the plugin at all.
	SyncStateMissing = "missing"
	// SyncStateUnsupportedPlatform means no executable ships for the node OS.
	SyncStateUnsupportedPlatform = "unsupported_platform"
	// SyncStateUnsupported means the node speaks another plugin api version.
	SyncStateUnsupported = "unsupported"
	// SyncStateOffline means the node was not reachable.
	SyncStateOffline = "offline"
	// SyncStateOptedOut means the operator excluded the node from plugin sync.
	SyncStateOptedOut = "opted_out"
	// SyncStateError means the node answered with an error.
	SyncStateError = "error"
)

const (
	// matrixTimeout bounds one node inspection so the matrix always renders.
	matrixTimeout = 5 * time.Second
	// syncTimeout bounds one node during a sync, which may upload a package.
	syncTimeout = 2 * time.Minute
	// reconcileInterval is how often every auto plugin is checked again.
	reconcileInterval = 10 * time.Minute
	// jobQueueSize bounds the pending sync requests. The engine is idempotent,
	// so a dropped request is picked up by the next reconcile pass.
	jobQueueSize = 64
)

// settingsRepushInterval is how long a reconcile pass trusts an unchanged
// settings push. After it the settings go out again, which repairs a node
// whose settings were edited locally or restored from a backup. Tests
// shorten it.
var settingsRepushInterval = time.Hour

// MatrixNode is one column of the matrix.
type MatrixNode struct {
	ID               uint64 `json:"id"`
	Name             string `json:"name"`
	URL              string `json:"url"`
	Online           bool   `json:"online"`
	AcceptPluginSync bool   `json:"accept_plugin_sync"`
	OS               string `json:"os,omitempty"`
	Arch             string `json:"arch,omitempty"`
}

// MatrixCell is the state of one plugin on one node.
type MatrixCell struct {
	NodeID  uint64 `json:"node_id"`
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status,omitempty"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// MatrixRow is one plugin together with its cluster sync intent.
type MatrixRow struct {
	PluginID     string       `json:"plugin_id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Enabled      bool         `json:"enabled"`
	APIVersion   int          `json:"api_version"`
	SyncPolicy   string       `json:"sync_policy"`
	SyncNodeIDs  []uint64     `json:"sync_node_ids"`
	SyncSettings bool         `json:"sync_settings"`
	Cells        []MatrixCell `json:"cells"`
}

// Matrix is the plugin inventory of the whole cluster.
type Matrix struct {
	Nodes []MatrixNode `json:"nodes"`
	Rows  []MatrixRow  `json:"rows"`
}

// NodeResult reports what one sync run did on one node.
type NodeResult struct {
	NodeID   uint64   `json:"node_id"`
	Node     string   `json:"node"`
	PluginID string   `json:"plugin_id"`
	Success  bool     `json:"success"`
	State    string   `json:"state"`
	Actions  []string `json:"actions"`
	Version  string   `json:"version,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// nodeRestyClient builds the authenticated client for one node. Tests replace
// it so a fake node does not need real cluster credentials.
var nodeRestyClient = func(node *model.Node) *resty.Client {
	return nodeauth.NewRestyClient(node)
}

// nodeStatusSnapshot reports which nodes the analytic monitor currently sees.
// Tests replace it so no monitor has to run.
var nodeStatusSnapshot = analytic.SnapshotNodeMap

// nodeClient talks to the plugin API of one child node.
type nodeClient struct {
	id     uint64
	name   string
	client *resty.Client
}

func newNodeClient(node *model.Node, timeout time.Duration) *nodeClient {
	client := nodeRestyClient(node)
	client.SetBaseURL(node.URL)
	client.SetTimeout(timeout)
	return &nodeClient{id: node.ID, name: node.Name, client: client}
}

// decode runs a request and unmarshals a 2xx body into out.
func decode(resp *resty.Response, err error, path string, out any) error {
	if err != nil {
		return err
	}
	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return fmt.Errorf("%s responded %d: %s", path, resp.StatusCode(), resp.String())
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Body(), out)
}

func (n *nodeClient) listPlugins(ctx context.Context) ([]Info, error) {
	var infos []Info
	resp, err := n.client.R().SetContext(ctx).Get("/api/plugins")
	if err = decode(resp, err, "/api/plugins", &infos); err != nil {
		return nil, err
	}
	return infos, nil
}

func (n *nodeClient) spec(ctx context.Context) (*Spec, error) {
	var spec Spec
	resp, err := n.client.R().SetContext(ctx).Get("/api/plugins/spec")
	if err = decode(resp, err, "/api/plugins/spec", &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// marketplaceInstall asks the node to pull the package itself, which keeps the
// controller from relaying megabytes it does not have to.
func (n *nodeClient) marketplaceInstall(ctx context.Context, id, version string, enable bool) error {
	body := map[string]any{
		"id":                  id,
		"version":             version,
		"enable":              enable,
		"approve_permissions": true,
		// The main node decides what runs, so a node makes room for it.
		"replace_conflicts": true,
	}
	resp, err := n.client.R().SetContext(ctx).SetBody(body).Post("/api/plugins/marketplace/install")
	return decode(resp, err, "/api/plugins/marketplace/install", nil)
}

// uploadPackage pushes the package the controller installed. authorKey is the
// key that made it community trust here, the node has no catalog entry to
// find it in.
func (n *nodeClient) uploadPackage(ctx context.Context, archivePath string, enable bool, authorKey string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	form := map[string]string{"enable": boolText(enable), "replace_conflicts": "true"}
	if authorKey != "" {
		form["author_public_key"] = authorKey
	}
	resp, err := n.client.R().SetContext(ctx).
		SetFileReader("file", "package.tar.gz", file).
		SetFormData(form).
		Post("/api/plugins")
	return decode(resp, err, "/api/plugins", nil)
}

func (n *nodeClient) setEnabled(ctx context.Context, id string, enabled bool) error {
	path := "/api/plugins/" + id + "/disable"
	request := n.client.R().SetContext(ctx)
	if enabled {
		path = "/api/plugins/" + id + "/enable"
		request = request.SetBody(map[string]any{"approve_permissions": true, "replace_conflicts": true})
	}
	resp, err := request.Post(path)
	return decode(resp, err, path, nil)
}

func (n *nodeClient) setChannel(ctx context.Context, id, channel string) error {
	path := "/api/plugins/" + id + "/channel"
	resp, err := n.client.R().SetContext(ctx).SetBody(map[string]any{"channel": channel}).Post(path)
	return decode(resp, err, path, nil)
}

func (n *nodeClient) saveSettings(ctx context.Context, id string, values map[string]any) error {
	path := "/api/plugins/" + id + "/settings"
	resp, err := n.client.R().SetContext(ctx).
		SetBody(map[string]any{"settings": values}).Post(path)
	return decode(resp, err, path, nil)
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// syncJob is one unit of work for the engine worker.
type syncJob struct {
	// pluginID is empty for a full reconcile pass.
	pluginID string
	// nodeID limits a node joined job to the node that just appeared.
	nodeID uint64
}

// Syncer keeps the plugins of the child nodes aligned with this controller.
// Every run is serialized through one worker so two passes never install the
// same plugin on the same node at the same time.
type Syncer struct {
	manager *Manager

	mu sync.RWMutex
	// results is the outcome of the last run per plugin and node.
	results map[string]map[uint64]NodeResult

	// runLocks serialises the runs of one plugin, so a manual sync and the
	// engine never push the same plugin to the same node at the same time.
	runLocks sync.Map
	// pushed remembers, per plugin and node, the last settings push, so a
	// reconcile pass leaves a node alone until the settings change or the
	// push is older than settingsRepushInterval.
	pushed map[string]map[uint64]settingsPush

	jobs chan syncJob

	engineMu    sync.Mutex
	cancel      context.CancelFunc
	unsubscribe func()
	done        chan struct{}
}

func newSyncer(manager *Manager) *Syncer {
	return &Syncer{
		manager: manager,
		results: map[string]map[uint64]NodeResult{},
		pushed:  map[string]map[uint64]settingsPush{},
		jobs:    make(chan syncJob, jobQueueSize),
	}
}

// Start brings the worker, the event subscription and the reconcile ticker up.
func (s *Syncer) Start(ctx context.Context) {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.cancel != nil {
		return
	}

	workerCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.unsubscribe = event.Subscribe(s.onEvent)

	go s.run(workerCtx, s.done)
}

// Stop tears the worker down and waits for the run in flight to finish.
func (s *Syncer) Stop() {
	s.engineMu.Lock()
	cancel, done, unsubscribe := s.cancel, s.done, s.unsubscribe
	s.cancel, s.done, s.unsubscribe = nil, nil, nil
	s.engineMu.Unlock()

	if unsubscribe != nil {
		unsubscribe()
	}
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

func (s *Syncer) run(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcile(ctx, 0)
		case job := <-s.jobs:
			s.process(ctx, job)
		}
	}
}

// onEvent turns the bus events into jobs. Bus subscribers must not block, so a
// full queue drops the job and lets the reconcile pass catch up.
func (s *Syncer) onEvent(published event.Event) {
	switch published.Type {
	case event.TypePluginChanged:
		data, ok := published.Data.(map[string]any)
		if !ok {
			return
		}
		id := cast.ToString(data["plugin_id"])
		if id == "" || cast.ToString(data["action"]) == ActionUninstalled {
			return
		}
		s.enqueue(syncJob{pluginID: id})
	case event.TypeNodeJoined:
		data, ok := published.Data.(map[string]any)
		if !ok {
			return
		}
		s.enqueue(syncJob{nodeID: cast.ToUint64(data["node_id"])})
	default:
	}
}

func (s *Syncer) enqueue(job syncJob) {
	select {
	case s.jobs <- job:
	default:
	}
}

// process runs one job. A job naming a plugin syncs only that plugin, a job
// naming a node syncs every auto plugin that targets it.
func (s *Syncer) process(ctx context.Context, job syncJob) {
	if job.pluginID == "" {
		s.reconcile(ctx, job.nodeID)
		return
	}

	row, ok := s.manager.syncRow(job.pluginID)
	if !ok || row.SyncPolicy != model.PluginSyncPolicyAuto {
		return
	}
	s.syncPlugin(ctx, job.pluginID, nil, false)
}

// reconcile syncs every auto plugin. A non zero nodeID limits the pass to the
// plugins whose target set covers that node.
func (s *Syncer) reconcile(ctx context.Context, nodeID uint64) {
	for _, id := range s.manager.autoPlugins() {
		if ctx.Err() != nil {
			return
		}
		if nodeID != 0 {
			row, ok := s.manager.syncRow(id)
			if !ok || (len(row.SyncNodeIDs) > 0 && !slices.Contains(row.SyncNodeIDs, nodeID)) {
				continue
			}
		}
		s.syncPlugin(ctx, id, nil, false)
	}
}

// SyncPlugin pushes one plugin to the given nodes. An empty nodeIDs falls back
// to the target set stored on the plugin, which in turn means every node.
func (s *Syncer) SyncPlugin(ctx context.Context, id string, nodeIDs []uint64) ([]NodeResult, error) {
	return s.syncPlugin(ctx, id, nodeIDs, true)
}

// lockPlugin blocks until this plugin has no other run in flight.
func (s *Syncer) lockPlugin(id string) func() {
	value, _ := s.runLocks.LoadOrStore(id, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (s *Syncer) syncPlugin(ctx context.Context, id string, nodeIDs []uint64, manual bool) ([]NodeResult, error) {
	unlock := s.lockPlugin(id)
	defer unlock()

	info, err := s.manager.Get(id)
	if err != nil {
		return nil, err
	}
	row, ok := s.manager.syncRow(id)
	if !ok {
		return nil, ErrPluginNotFound
	}
	if len(nodeIDs) == 0 {
		nodeIDs = row.SyncNodeIDs
	}

	nodes, err := targetNodes(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}

	archive, cleanup, archiveErr := s.manager.EnsureArchive(id)
	defer cleanup()
	own := &ownArchive{path: archive, err: archiveErr, platforms: s.manager.installedPlatforms(id)}
	statuses := nodeStatusSnapshot()

	results := make([]NodeResult, len(nodes))
	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))
	for index, node := range nodes {
		go func(index int, node *model.Node) {
			defer wg.Done()
			results[index] = s.syncNode(ctx, node, info, row, own, statusPlatform(statuses, node.ID), manual)
		}(index, node)
	}
	wg.Wait()

	sort.SliceStable(results, func(i, j int) bool { return results[i].Node < results[j].Node })
	s.storeResults(id, results)
	s.notify(info.Name, results, manual)
	return results, nil
}

// syncNode aligns one plugin on one node. Every step reports through the
// result instead of aborting the whole run. platform is what the node monitor
// reported, the node spec overrides it when the node advertises one. A manual
// run pushes the settings even when nothing changed since the last push.
func (s *Syncer) syncNode(
	ctx context.Context,
	node *model.Node,
	info *Info,
	row *model.Plugin,
	own *ownArchive,
	platform string,
	manual bool,
) NodeResult {
	result := NodeResult{
		NodeID:   node.ID,
		Node:     node.Name,
		PluginID: info.ID,
		Actions:  []string{},
		Version:  info.Version,
	}
	if !node.AcceptPluginSync {
		result.State = SyncStateOptedOut
		result.Success = true
		return result
	}

	nodeCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	client := newNodeClient(node, syncTimeout)
	spec, err := client.spec(nodeCtx)
	if err != nil {
		result.State = SyncStateOffline
		result.Error = err.Error()
		return result
	}
	if info.APIVersion != 0 && len(spec.APIVersions) > 0 && !slices.Contains(spec.APIVersions, info.APIVersion) {
		result.State = SyncStateUnsupported
		result.Error = fmt.Sprintf("node speaks plugin api versions %v", spec.APIVersions)
		return result
	}
	if spec.Platform != "" {
		platform = spec.Platform
	}

	infos, err := client.listPlugins(nodeCtx)
	if err != nil {
		result.State = SyncStateOffline
		result.Error = err.Error()
		return result
	}
	remote := findInfo(infos, info.ID)
	// A node that only now gets the package holds no settings of the plugin,
	// whatever was pushed to it before.
	installedNow := remote == nil

	if remote == nil || remote.Version != info.Version {
		action := "installed"
		if remote != nil {
			action = "updated"
		}
		if err = s.installOnNode(nodeCtx, client, info, row.AuthorPublicKey, own, platform); err != nil {
			result.State = SyncStateError
			if errors.Is(err, errNoPlatformPackage) {
				result.State = SyncStateUnsupportedPlatform
			}
			result.Error = err.Error()
			return result
		}
		result.Actions = append(result.Actions, action)
		// The install already applied the wanted enabled state.
		// An install leaves the chosen channel alone, a new plugin starts on stable.
		afterInstall := &Info{ID: info.ID, Version: info.Version, Enabled: info.Enabled}
		if remote == nil {
			afterInstall.FollowedChannel = ChannelStable
		} else {
			afterInstall.FollowedChannel = remote.FollowedChannel
		}
		remote = afterInstall
	}

	if remote.Enabled != info.Enabled {
		if err = client.setEnabled(nodeCtx, info.ID, info.Enabled); err != nil {
			result.State = SyncStateError
			result.Error = err.Error()
			return result
		}
		result.Actions = append(result.Actions, enabledAction(info.Enabled))
	}

	// A node that does not report a channel predates channels.
	if followed := NormalizeChannel(row.FollowedChannel); remote.FollowedChannel != "" && remote.FollowedChannel != followed {
		if err = client.setChannel(nodeCtx, info.ID, followed); err != nil {
			result.State = SyncStateError
			result.Error = err.Error()
			return result
		}
		result.Actions = append(result.Actions, "channel")
	}

	if row.SyncSettings && len(row.Settings) > 0 {
		digest := settingsDigest(row.Settings)
		if manual || installedNow || digest == "" || s.settingsPushDue(info.ID, node.ID, digest) {
			if err = client.saveSettings(nodeCtx, info.ID, row.Settings); err != nil {
				result.State = SyncStateError
				result.Error = err.Error()
				return result
			}
			s.rememberPushed(info.ID, node.ID, digest)
			result.Actions = append(result.Actions, "settings")
		}
	}

	result.State = SyncStateInSync
	result.Success = true
	return result
}

// installOnNode prefers the marketplace so the node fetches the package that
// matches its own platform from the catalog, and falls back to pushing a
// package for that platform, see pushArchive, together with the author key.
func (s *Syncer) installOnNode(ctx context.Context, client *nodeClient, info *Info, authorKey string,
	own *ownArchive, platform string,
) error {
	marketplaceErr := client.marketplaceInstall(ctx, info.ID, info.Version, info.Enabled)
	if marketplaceErr == nil {
		return nil
	}
	archive, err := s.manager.pushArchive(ctx, info, own, platform)
	if err != nil {
		return fmt.Errorf("marketplace install failed (%v) and no package is available: %w", marketplaceErr, err)
	}
	if err = client.uploadPackage(ctx, archive, info.Enabled, authorKey); err != nil {
		return fmt.Errorf("marketplace install failed (%v) and pushing the package failed: %w", marketplaceErr, err)
	}
	return nil
}

func enabledAction(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func findInfo(infos []Info, id string) *Info {
	for index := range infos {
		if infos[index].ID == id {
			return &infos[index]
		}
	}
	return nil
}

// SetPolicy persists the sync intent of a plugin and pushes it right away when
// it was switched to automatic.
func (s *Syncer) SetPolicy(ctx context.Context, id, policy string, nodeIDs []uint64, syncSettings bool) error {
	if policy != model.PluginSyncPolicyAuto {
		policy = model.PluginSyncPolicyManual
	}
	if err := s.manager.setSyncPolicy(ctx, id, policy, nodeIDs, syncSettings); err != nil {
		return err
	}
	if policy == model.PluginSyncPolicyAuto {
		s.enqueue(syncJob{pluginID: id})
	}
	return nil
}

// targetNodes resolves the enabled nodes a run applies to. An empty nodeIDs
// means every enabled node, including the ones added later.
func targetNodes(ctx context.Context, nodeIDs []uint64) ([]*model.Node, error) {
	n := query.Node
	stmt := n.WithContext(ctx).Where(n.Enabled.Is(true))
	if len(nodeIDs) > 0 {
		stmt = stmt.Where(n.ID.In(nodeIDs...))
	}
	return stmt.Order(n.Name).Find()
}

func (s *Syncer) storeResults(id string, results []NodeResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byNode := make(map[uint64]NodeResult, len(results))
	for _, result := range results {
		byNode[result.NodeID] = result
	}
	s.results[id] = byNode
}

// settingsDigest fingerprints a settings map. json sorts the keys, so equal
// maps always produce the same digest.
func settingsDigest(values map[string]any) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// settingsPush is one successful settings push to a node.
type settingsPush struct {
	digest string
	at     time.Time
}

// settingsPushDue reports whether a background pass pushes the settings to a
// node: they changed since the last push, or that push is too old.
func (s *Syncer) settingsPushDue(id string, nodeID uint64, digest string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	last, ok := s.pushed[id][nodeID]
	return !ok || last.digest != digest || time.Since(last.at) >= settingsRepushInterval
}

func (s *Syncer) rememberPushed(id string, nodeID uint64, digest string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byNode, ok := s.pushed[id]
	if !ok {
		byNode = map[uint64]settingsPush{}
		s.pushed[id] = byNode
	}
	byNode[nodeID] = settingsPush{digest: digest, at: time.Now()}
}

func (s *Syncer) lastResult(id string, nodeID uint64) (NodeResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byNode, ok := s.results[id]
	if !ok {
		return NodeResult{}, false
	}
	result, ok := byNode[nodeID]
	return result, ok
}

// notify summarises a run. A background pass that changed nothing stays quiet.
func (s *Syncer) notify(name string, results []NodeResult, manual bool) {
	failed, changed := 0, 0
	for _, result := range results {
		if !result.Success {
			failed++
			continue
		}
		if len(result.Actions) > 0 {
			changed++
		}
	}

	if failed > 0 {
		notification.Warning("Plugin Cluster Sync Failed",
			"Plugin %{name} could not be synchronized to %{failed} of %{total} node(s)",
			map[string]any{"name": name, "failed": failed, "total": len(results)})
		return
	}
	if changed == 0 && !manual {
		return
	}
	notification.Info("Plugin Cluster Sync",
		"Plugin %{name} is in sync on %{total} node(s)",
		map[string]any{"name": name, "total": len(results)})
}

// Matrix reports every installed plugin against every enabled node.
func (s *Syncer) Matrix(ctx context.Context) (*Matrix, error) {
	nodes, err := targetNodes(ctx, nil)
	if err != nil {
		return nil, err
	}

	statuses := nodeStatusSnapshot()
	columns := make([]MatrixNode, len(nodes))
	states := make([]nodeInventory, len(nodes))

	// The catalog tells whether a node on another platform can still get a
	// package. It is only read when such a node shows up.
	catalog := &lazyCatalog{load: func() []CatalogEntry { return s.manager.matrixCatalog(ctx) }}

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))
	for index, node := range nodes {
		column := MatrixNode{
			ID:               node.ID,
			Name:             node.Name,
			URL:              node.URL,
			AcceptPluginSync: node.AcceptPluginSync,
		}
		if status, ok := statuses[node.ID]; ok && status != nil {
			column.Online = status.Status
			column.OS = status.NodeRuntimeInfo.OS
			column.Arch = status.NodeRuntimeInfo.Arch
		}
		columns[index] = column

		go func(index int, node *model.Node, column MatrixNode) {
			defer wg.Done()
			states[index] = inspectNode(ctx, node, column)
		}(index, node, column)
	}
	wg.Wait()

	// A node the monitor has not described yet still names its platform in
	// the plugin spec.
	for index := range columns {
		if columns[index].platform() != "" || states[index].spec == nil {
			continue
		}
		if goos, goarch, ok := strings.Cut(states[index].spec.Platform, "-"); ok {
			columns[index].OS, columns[index].Arch = goos, goarch
		}
	}

	infos := s.manager.List()
	rows := make([]MatrixRow, 0, len(infos))
	for _, info := range infos {
		manifest, _ := s.manager.Manifest(info.ID)
		row := MatrixRow{
			PluginID:     info.ID,
			Name:         info.Name,
			Version:      info.Version,
			Enabled:      info.Enabled,
			APIVersion:   info.APIVersion,
			SyncPolicy:   info.SyncPolicy,
			SyncNodeIDs:  info.SyncNodeIDs,
			SyncSettings: info.SyncSettings,
			Cells:        make([]MatrixCell, 0, len(columns)),
		}
		availability := s.manager.newPlatformAvailability(info, manifest, catalog)
		for index, column := range columns {
			row.Cells = append(row.Cells, s.cellOf(info, availability, column, states[index]))
		}
		rows = append(rows, row)
	}

	return &Matrix{Nodes: columns, Rows: rows}, nil
}

// nodeInventory is what one node answered while the matrix was built.
type nodeInventory struct {
	reachable bool
	optedOut  bool
	offline   bool
	err       string
	infos     []Info
	spec      *Spec
}

func inspectNode(ctx context.Context, node *model.Node, column MatrixNode) nodeInventory {
	if !column.AcceptPluginSync {
		return nodeInventory{optedOut: true}
	}
	if !column.Online {
		return nodeInventory{offline: true}
	}

	nodeCtx, cancel := context.WithTimeout(ctx, matrixTimeout)
	defer cancel()

	client := newNodeClient(node, matrixTimeout)
	infos, err := client.listPlugins(nodeCtx)
	if err != nil {
		return nodeInventory{err: err.Error()}
	}
	inventory := nodeInventory{reachable: true, infos: infos}
	// A node that cannot report its spec still shows its inventory.
	if spec, specErr := client.spec(nodeCtx); specErr == nil {
		inventory.spec = spec
	}
	return inventory
}

// cellOf folds one node answer and the local plugin into a matrix cell.
func (s *Syncer) cellOf(info Info, availability *platformAvailability, column MatrixNode, inventory nodeInventory) MatrixCell {
	cell := MatrixCell{NodeID: column.ID}
	switch {
	case inventory.optedOut:
		cell.State = SyncStateOptedOut
		return cell
	case inventory.offline:
		cell.State = SyncStateOffline
		return cell
	case !inventory.reachable:
		cell.State = SyncStateError
		cell.Message = inventory.err
		return cell
	}

	remote := findInfo(inventory.infos, info.ID)
	// A node already running this version evidently has a build for it.
	if remote == nil || remote.Version != info.Version {
		platform := column.platform()
		if inventory.spec != nil && inventory.spec.Platform != "" {
			platform = inventory.spec.Platform
		}
		if reason, unsupported := availability.unsupported(platform); unsupported {
			cell.State = SyncStateUnsupportedPlatform
			cell.Message = reason
			return cell
		}
	}
	if inventory.spec != nil && info.APIVersion != 0 && len(inventory.spec.APIVersions) > 0 &&
		!slices.Contains(inventory.spec.APIVersions, info.APIVersion) {
		cell.State = SyncStateUnsupported
		cell.Message = fmt.Sprintf("node speaks plugin api versions %v", inventory.spec.APIVersions)
		return cell
	}

	if remote == nil {
		cell.State = SyncStateMissing
		if result, ok := s.lastResult(info.ID, column.ID); ok && !result.Success {
			cell.Message = result.Error
		}
		return cell
	}

	cell.Version = remote.Version
	cell.Enabled = remote.Enabled
	cell.Status = string(remote.Status)
	switch {
	case remote.Version != info.Version:
		cell.State = SyncStateOutdated
	case remote.Enabled != info.Enabled:
		cell.State = SyncStateOutdated
		cell.Message = "enabled state differs"
	default:
		cell.State = SyncStateInSync
	}
	if result, ok := s.lastResult(info.ID, column.ID); ok && !result.Success && cell.Message == "" {
		cell.Message = result.Error
	}
	return cell
}
