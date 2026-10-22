package blocklist

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gen/field"
)

const (
	// DirName is the directory below the nginx configuration directory that
	// holds one file per source.
	DirName = "blocklists"
	// defaultTick is how often the runner looks for due sources.
	defaultTick = 30 * time.Second
	// retryAfterFailure bounds the wait before a failed fetch is retried.
	retryAfterFailure = 5 * time.Minute
)

// Writer writes the generated files and reloads nginx when one changed.
// config.GeneratedWriter is the implementation outside of tests.
type Writer interface {
	Apply(path string, content []byte) (changed bool, err error)
	Remove(path string) (removed bool, err error)
}

// Runner refreshes the blocklist sources and writes their files.
type Runner struct {
	ctx     context.Context
	writer  Writer
	confDir func() string
	now     func() time.Time
	tick    time.Duration

	mu sync.Mutex
	// locks serializes the runs of one source.
	locks map[uint64]*sync.Mutex
}

// Option configures a Runner.
type Option func(*Runner)

// WithWriter replaces the writer that uses the nginx of the host, for tests.
func WithWriter(writer Writer) Option {
	return func(r *Runner) { r.writer = writer }
}

// WithConfDir replaces the nginx configuration directory, for tests.
func WithConfDir(confDir func() string) Option {
	return func(r *Runner) { r.confDir = confDir }
}

// WithClock replaces the clock, for tests.
func WithClock(now func() time.Time) Option {
	return func(r *Runner) { r.now = now }
}

// NewRunner returns a runner whose runs stop when ctx ends.
func NewRunner(ctx context.Context, opts ...Option) *Runner {
	r := &Runner{
		ctx:     ctx,
		writer:  config.GeneratedWriter{},
		confDir: func() string { return nginx.GetConfPath() },
		now:     time.Now,
		tick:    defaultTick,
		locks:   map[uint64]*sync.Mutex{},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

var defaultRunner atomic.Pointer[Runner]

// Default returns the runner Start created, or one that uses the nginx of
// the host when Start was not called.
func Default() *Runner {
	if r := defaultRunner.Load(); r != nil {
		return r
	}
	defaultRunner.CompareAndSwap(nil, NewRunner(context.Background()))
	return defaultRunner.Load()
}

// SetDefault makes r the runner the API uses.
func SetDefault(r *Runner) {
	defaultRunner.Store(r)
}

// Start creates the default runner and refreshes every due source on a
// 30 second tick until ctx ends.
func Start(ctx context.Context) *Runner {
	r := NewRunner(ctx)
	SetDefault(r)

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		logger.Errorf("Create the blocklist scheduler: %v", err)
		return r
	}
	_, err = scheduler.NewJob(
		gocron.DurationJob(r.tick),
		gocron.NewTask(r.RefreshDue),
		gocron.WithName("blocklist_refresh"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		logger.Errorf("Schedule the blocklist refresh: %v", err)
		return r
	}
	scheduler.Start()
	go func() {
		<-ctx.Done()
		_ = scheduler.Shutdown()
	}()
	return r
}

// Path is the file of a source.
func (r *Runner) Path(id uint64) string {
	return filepath.Join(r.confDir(), DirName, strconv.FormatUint(id, 10)+".conf")
}

// IncludePath is the file of a source relative to the directory of
// nginx.conf, which nginx resolves a relative include against.
func IncludePath(id uint64) string {
	return DirName + "/" + strconv.FormatUint(id, 10) + ".conf"
}

// IncludeLine is the directive a person adds where the list should apply.
func IncludeLine(id uint64) string {
	return "include " + IncludePath(id) + ";"
}

func (r *Runner) lockOf(id uint64) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock, ok := r.locks[id]
	if !ok {
		lock = &sync.Mutex{}
		r.locks[id] = lock
	}
	return lock
}

// RefreshDue refreshes every enabled source whose time has come, one run per
// source at a time, and waits for them. A source that is running already is
// left to that run.
func (r *Runner) RefreshDue() {
	if r.ctx.Err() != nil {
		return
	}
	q := query.BlocklistSource
	enabled, err := q.Where(q.Enabled.Is(true)).Order(q.ID).Find()
	if err != nil {
		logger.Errorf("Failed to load the blocklist sources: %v", err)
		return
	}

	now := r.now()
	var wg sync.WaitGroup
	for _, source := range enabled {
		if source.NextRunAt != nil && source.NextRunAt.After(now) {
			continue
		}
		lock := r.lockOf(source.ID)
		if !lock.TryLock() {
			continue
		}
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			defer lock.Unlock()
			if _, err := r.run(r.ctx, id); err != nil {
				logger.Warnf("Failed to refresh blocklist source %d: %v", id, err)
			}
		}(source.ID)
	}
	wg.Wait()
}

// Refresh fetches a source now, due, disabled or not, since the person asked
// for it, and returns it with the outcome recorded.
func (r *Runner) Refresh(ctx context.Context, id uint64) (*model.BlocklistSource, error) {
	lock := r.lockOf(id)
	lock.Lock()
	defer lock.Unlock()
	return r.run(ctx, id)
}

// Remove deletes the file of a source and reloads nginx. It refuses while
// the nginx configuration still includes the file, which nginx would no
// longer accept.
func (r *Runner) Remove(id uint64) error {
	lock := r.lockOf(id)
	lock.Lock()
	defer lock.Unlock()

	if _, err := r.writer.Remove(r.Path(id)); err != nil {
		if testErr, ok := errors.AsType[*config.TestError](err); ok {
			return cosy.WrapErrorWithParams(plugin.ErrGeneratedFileIncluded, IncludePath(id), testErr.Err.Error())
		}
		return err
	}
	return nil
}

// outcome is the result of one run.
type outcome struct {
	status  string
	message string
	// entries is the number of deny rules now in the file, -1 when the file
	// was left as it was.
	entries int
	// ttl is what the source asked for, 0 for no opinion.
	ttl int
}

// run refreshes one source and records the outcome.
func (r *Runner) run(ctx context.Context, id uint64) (*model.BlocklistSource, error) {
	q := query.BlocklistSource
	source, err := q.Where(q.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}

	started := r.now()
	result := r.refresh(ctx, source)
	if ctx.Err() != nil && result.status == model.RefreshStatusFailed {
		// The host is shutting down, the next start runs the source again.
		return source, nil
	}

	next := started.Add(nextInterval(source.RefreshSeconds, result.ttl, result.status == model.RefreshStatusFailed))
	updates := []field.AssignExpr{
		q.LastRunAt.Value(started),
		q.NextRunAt.Value(next),
		q.LastStatus.Value(result.status),
		q.LastMessage.Value(result.message),
	}
	if result.entries >= 0 {
		updates = append(updates, q.EntryCount.Value(result.entries))
	}
	if _, err = q.Where(q.ID.Eq(id)).UpdateColumnSimple(updates...); err != nil {
		return nil, err
	}
	return q.Where(q.ID.Eq(id)).First()
}

// refresh fetches the list of a source and writes its file.
func (r *Runner) refresh(ctx context.Context, source *model.BlocklistSource) outcome {
	result, err := Fetch(ctx, source.Kind, source.Config)
	if err != nil {
		return outcome{status: model.RefreshStatusFailed, message: err.Error(), entries: -1}
	}

	rendered := Render(result.Entries)
	changed, err := r.writer.Apply(r.Path(source.ID), rendered.Content)
	if err != nil {
		return outcome{
			status:  model.RefreshStatusFailed,
			message: fmt.Sprintf("Fetched %d entries but could not apply them: %v", rendered.Written, err),
			entries: -1,
			ttl:     result.TTLSeconds,
		}
	}
	return outcome{
		status:  model.RefreshStatusOK,
		message: summary(rendered, changed),
		entries: rendered.Written,
		ttl:     result.TTLSeconds,
	}
}

// summary describes a successful run.
func summary(rendered Rendered, changed bool) string {
	parts := []string{fmt.Sprintf("%d entries written", rendered.Written)}
	if rendered.Dropped > 0 {
		parts = append(parts, fmt.Sprintf("%d invalid entries dropped", rendered.Dropped))
	}
	if rendered.Duplicates > 0 {
		parts = append(parts, fmt.Sprintf("%d duplicates removed", rendered.Duplicates))
	}
	if changed {
		parts = append(parts, "nginx reloaded")
	} else {
		parts = append(parts, "no change")
	}
	return strings.Join(parts, ", ")
}

// nextInterval is how long a source waits before its next run: its own
// interval, shortened by a shorter ttl of the source, never below the
// minimum, and at most retryAfterFailure after a failure.
func nextInterval(refreshSeconds, ttlSeconds int, failed bool) time.Duration {
	seconds := refreshSeconds
	if seconds <= 0 {
		seconds = protocol.DefaultBlocklistRefreshSeconds
	}
	if ttlSeconds > 0 && ttlSeconds < seconds {
		seconds = ttlSeconds
	}
	seconds = max(seconds, protocol.MinBlocklistRefreshSeconds)
	interval := time.Duration(seconds) * time.Second
	if failed && interval > retryAfterFailure {
		interval = retryAfterFailure
	}
	return interval
}
