package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// cronCallTimeout bounds one scheduled plugin invocation.
const cronCallTimeout = 10 * time.Minute

// defaultLocale is reported to plugins because nginx-ui keeps the interface
// language per user, not per node.
const defaultLocale = "en"

// hostBackend implements HostBackend on top of the database, the notification
// centre and the manager owned scheduler.
type hostBackend struct {
	manager *Manager
}

// hostBackend returns the host API implementation of this manager.
func (m *Manager) hostBackend() HostBackend { return m.backend }

// context returns the lifetime the manager was started with.
func (m *Manager) context() context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

// Log forwards a plugin log line to the host logger.
func (b *hostBackend) Log(pluginID string, p protocol.HostLogParams) {
	log := b.manager.log
	message := fmt.Sprintf("[plugin:%s] %s", pluginID, p.Message)
	if len(p.Fields) > 0 {
		if encoded, err := json.Marshal(p.Fields); err == nil {
			message += " " + string(encoded)
		}
	}

	switch strings.ToLower(p.Level) {
	case "debug":
		log.Debug(message)
	case "warn", "warning":
		log.Warn(message)
	case "error", "fatal":
		log.Error(message)
	default:
		log.Info(message)
	}
}

// KVGet reads one entry of the private plugin store.
func (b *hostBackend) KVGet(pluginID, key string) (json.RawMessage, bool, error) {
	row, err := b.kvRow(pluginID, key)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return row.Value, true, nil
}

// KVSet stores one entry, replacing what is there.
func (b *hostBackend) KVSet(pluginID, key string, value json.RawMessage) error {
	ctx := b.manager.context()
	row, err := b.kvRow(pluginID, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return query.PluginKV.WithContext(ctx).Create(&model.PluginKV{
			PluginID: pluginID,
			Key:      key,
			Value:    value,
		})
	}
	if err != nil {
		return err
	}
	row.Value = value
	return query.PluginKV.WithContext(ctx).Save(row)
}

// KVDelete removes one entry. The row is hard deleted so the unique index on
// (plugin_id, key) frees the pair again.
func (b *hostBackend) KVDelete(pluginID, key string) error {
	_, err := query.PluginKV.WithContext(b.manager.context()).Unscoped().
		Where(query.PluginKV.PluginID.Eq(pluginID), query.PluginKV.Key.Eq(key)).
		Delete()
	return err
}

// KVList returns the keys of a plugin, optionally narrowed by a prefix. The
// databases disagree on how LIKE escapes its pattern characters and on case,
// so LIKE only narrows the scan for a prefix without them and Go decides the
// match.
func (b *hostBackend) KVList(pluginID, prefix string) ([]string, error) {
	q := query.PluginKV.WithContext(b.manager.context()).
		Where(query.PluginKV.PluginID.Eq(pluginID))
	if prefix != "" && !strings.ContainsAny(prefix, `%_\`) {
		q = q.Where(query.PluginKV.Key.Like(prefix + "%"))
	}
	rows, err := q.Order(query.PluginKV.Key).Find()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.HasPrefix(row.Key, prefix) {
			keys = append(keys, row.Key)
		}
	}
	return keys, nil
}

func (b *hostBackend) kvRow(pluginID, key string) (*model.PluginKV, error) {
	// A missing key is a normal answer, so Find is used: First would log it as
	// an error.
	rows, err := query.PluginKV.WithContext(b.manager.context()).
		Where(query.PluginKV.PluginID.Eq(pluginID), query.PluginKV.Key.Eq(key)).
		Limit(1).
		Find()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return rows[0], nil
}

// SettingsGet returns the stored settings layered over the schema defaults.
func (b *hostBackend) SettingsGet(pluginID string) (map[string]any, error) {
	item, ok := b.manager.lookup(pluginID)
	if !ok {
		return nil, ErrPluginNotFound
	}
	b.manager.mu.RLock()
	defer b.manager.mu.RUnlock()
	return mergedSettings(item.manifest, item.row), nil
}

// Locale reports the interface language of the node. nginx-ui stores the
// language per user, so plugins get the neutral default.
func (b *hostBackend) Locale() string { return defaultLocale }

// CredentialsGet hands a stored DNS credential to a plugin that was granted
// credentials.read:dns.
func (b *hostBackend) CredentialsGet(pluginID, kind, id string) (*protocol.HostCredentialsGetResult, error) {
	if kind != "dns" {
		return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "unknown credential kind "+kind)
	}

	credentialID := cast.ToUint64(id)
	if credentialID == 0 {
		return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "id must be a credential id")
	}

	credential, err := query.DnsCredential.WithContext(b.manager.context()).
		Where(query.DnsCredential.ID.Eq(credentialID)).First()
	if err != nil {
		return nil, jsonrpc.Errorf(protocol.CodeInvalidConfig, "credential not found")
	}

	config := map[string]string{}
	if credential.Config != nil && credential.Config.Configuration != nil {
		for key, value := range credential.Config.Configuration.Credentials {
			config[key] = value
		}
		for key, value := range credential.Config.Configuration.Additional {
			config[key] = value
		}
	}

	providerCode := credential.ProviderCode
	if providerCode == "" && credential.Config != nil {
		providerCode = credential.Config.Code
	}
	return &protocol.HostCredentialsGetResult{
		ID:           id,
		Name:         credential.Name,
		ProviderCode: providerCode,
		Config:       config,
	}, nil
}

// Notify raises a notification in the nginx-ui notification centre.
func (b *hostBackend) Notify(pluginID string, p protocol.HostNotifyParams) error {
	title := p.Title
	if title == "" {
		title = pluginID
	}
	switch strings.ToLower(p.Level) {
	case "success":
		notification.Success(title, p.Content, p.Details)
	case "warn", "warning":
		notification.Warning(title, p.Content, p.Details)
	case "error":
		notification.Error(title, p.Content, p.Details)
	default:
		notification.Info(title, p.Content, p.Details)
	}
	return nil
}

// MetricsSnapshot returns the current node statistics. analytic exposes no
// cached snapshot, GetNodeStat samples the host for one second per call.
func (b *hostBackend) MetricsSnapshot() (any, error) {
	return analytic.GetNodeStat(), nil
}

// CronRegister adds a job a plugin asked for at runtime.
func (b *hostBackend) CronRegister(pluginID string, p protocol.HostCronRegisterParams) error {
	item, ok := b.manager.lookup(pluginID)
	if !ok {
		return ErrPluginNotFound
	}
	if err := b.manager.addCronJob(item, p.ID, p.Schedule, p.Method); err != nil {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, err.Error())
	}
	return nil
}

// CronUnregister removes a job a plugin registered earlier.
func (b *hostBackend) CronUnregister(pluginID, id string) error {
	item, ok := b.manager.lookup(pluginID)
	if !ok {
		return ErrPluginNotFound
	}
	b.manager.removeCronJob(item, id)
	return nil
}

// startScheduler creates the scheduler the manifest and host API cron entries
// run on. The caller holds opMu.
func (m *Manager) startScheduler() {
	m.mu.RLock()
	running := m.scheduler != nil
	m.mu.RUnlock()
	if running {
		return
	}

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		m.log.Errorf("Create plugin scheduler: %v", err)
		return
	}
	m.mu.Lock()
	m.scheduler = scheduler
	m.mu.Unlock()
	scheduler.Start()
}

// registerManifestCron adds every cron entry the manifest declares.
func (m *Manager) registerManifestCron(item *entry) {
	m.mu.RLock()
	manifest := item.manifest
	m.mu.RUnlock()
	if manifest == nil {
		return
	}
	for _, task := range manifest.Cron {
		if err := m.addCronJob(item, task.ID, task.Schedule, task.Method); err != nil {
			m.log.Warnf("[plugin:%s] cron %s: %v", item.id, task.ID, err)
		}
	}
}

// addCronJob schedules one plugin task, replacing a job with the same id.
func (m *Manager) addCronJob(item *entry, id, schedule, method string) error {
	if id == "" || method == "" {
		return errors.New("cron id and method are required")
	}

	m.mu.RLock()
	scheduler := m.scheduler
	m.mu.RUnlock()
	if scheduler == nil {
		return errors.New("the plugin scheduler is not running")
	}

	definition, err := cronDefinition(schedule)
	if err != nil {
		return err
	}

	m.removeCronJob(item, id)
	pluginID := item.id
	job, err := scheduler.NewJob(definition,
		gocron.NewTask(func() { m.runCronJob(pluginID, id, method) }),
		gocron.WithName(fmt.Sprintf("plugin_%s_%s", pluginID, id)),
		// One plugin task never overlaps itself.
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}

	m.mu.Lock()
	if item.cronJobs == nil {
		item.cronJobs = map[string]gocron.Job{}
	}
	item.cronJobs[id] = job
	m.mu.Unlock()
	return nil
}

// removeCronJob drops one scheduled task of a plugin.
func (m *Manager) removeCronJob(item *entry, id string) {
	m.mu.Lock()
	job, ok := item.cronJobs[id]
	if ok {
		delete(item.cronJobs, id)
	}
	scheduler := m.scheduler
	m.mu.Unlock()

	if !ok || scheduler == nil {
		return
	}
	if err := scheduler.RemoveJob(job.ID()); err != nil {
		m.log.Debugf("[plugin:%s] remove cron %s: %v", item.id, id, err)
	}
}

// unregisterCron drops every scheduled task of a plugin.
func (m *Manager) unregisterCron(item *entry) {
	m.mu.RLock()
	ids := make([]string, 0, len(item.cronJobs))
	for id := range item.cronJobs {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		m.removeCronJob(item, id)
	}
}

// minCronInterval is the shortest "@every" period a plugin may schedule, so
// a typo cannot turn a scheduled task into a busy loop.
const minCronInterval = time.Second

// cronDefinition understands five field cron expressions and "@every <duration>".
func cronDefinition(schedule string) (gocron.JobDefinition, error) {
	schedule = strings.TrimSpace(schedule)
	if rest, ok := strings.CutPrefix(schedule, "@every "); ok {
		interval, err := time.ParseDuration(strings.TrimSpace(rest))
		if err != nil {
			return nil, fmt.Errorf("parse interval %q: %w", rest, err)
		}
		if interval < minCronInterval {
			return nil, fmt.Errorf("interval %q is shorter than %s", rest, minCronInterval)
		}
		return gocron.DurationJob(interval), nil
	}
	if schedule == "" {
		return nil, errors.New("schedule is required")
	}
	if fields := strings.Fields(schedule); len(fields) != 5 {
		return nil, fmt.Errorf("schedule %q must have five fields", schedule)
	}
	return gocron.CronJob(schedule, false), nil
}

// runCronJob invokes the plugin method behind one scheduled task.
func (m *Manager) runCronJob(pluginID, id, method string) {
	ctx, cancel := context.WithTimeout(m.context(), cronCallTimeout)
	defer cancel()

	client, release, err := m.Acquire(ctx, pluginID)
	if err != nil {
		m.log.Warnf("[plugin:%s] cron %s: %v", pluginID, id, err)
		return
	}
	defer release()

	params := protocol.EventNotification{Type: id, TS: time.Now().Unix()}
	if err = client.Call(ctx, method, params, nil); err != nil {
		m.log.Warnf("[plugin:%s] cron %s: %v", pluginID, id, WrapRPCError(err))
	}
}
