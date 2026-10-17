package discovery

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gen/field"
)

const (
	// DirName is the directory below the nginx configuration directory that
	// holds one file per binding.
	DirName = "upstreams"
	// DefaultRefreshSeconds is the interval of a binding that sets none.
	DefaultRefreshSeconds = 60
	// MinRefreshSeconds is the shortest interval of a binding.
	MinRefreshSeconds = 10
	// defaultTick is how often the runner looks for due bindings.
	defaultTick = 10 * time.Second
	// retryAfterFailure bounds the wait before a failed resolution is retried.
	retryAfterFailure = 5 * time.Minute
)

// Writer writes the generated files and reloads nginx when one changed.
// config.GeneratedWriter is the implementation outside of tests.
type Writer interface {
	Apply(path string, content []byte) (changed bool, err error)
	Remove(path string) (removed bool, err error)
}

// Runner resolves the upstream discoveries and writes their files.
type Runner struct {
	ctx     context.Context
	writer  Writer
	confDir func() string
	now     func() time.Time
	tick    time.Duration

	mu sync.Mutex
	// locks serializes the runs of one binding.
	locks map[uint64]*sync.Mutex
	// files serializes the writes of one upstream file across bindings.
	files sync.Mutex
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

// Start creates the default runner and resolves every due binding on a
// 10 second tick until ctx ends.
func Start(ctx context.Context) *Runner {
	r := NewRunner(ctx)
	SetDefault(r)

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		logger.Errorf("Create the upstream discovery scheduler: %v", err)
		return r
	}
	_, err = scheduler.NewJob(
		gocron.DurationJob(r.tick),
		gocron.NewTask(r.RefreshDue),
		gocron.WithName("upstream_discovery_refresh"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		logger.Errorf("Schedule the upstream discovery refresh: %v", err)
		return r
	}
	scheduler.Start()
	go func() {
		<-ctx.Done()
		_ = scheduler.Shutdown()
	}()
	return r
}

// Path is the file of an upstream.
func (r *Runner) Path(upstreamName string) string {
	return filepath.Join(r.confDir(), DirName, upstreamName+".conf")
}

// IncludePath is the file of an upstream relative to the directory of
// nginx.conf, which nginx resolves a relative include against.
func IncludePath(upstreamName string) string {
	return DirName + "/" + upstreamName + ".conf"
}

// IncludeLine is the directive a person adds to the http block.
func IncludeLine(upstreamName string) string {
	return "include " + IncludePath(upstreamName) + ";"
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

// RefreshDue resolves every enabled binding whose time has come, one run per
// binding at a time, and waits for them.
func (r *Runner) RefreshDue() {
	if r.ctx.Err() != nil {
		return
	}
	q := query.UpstreamDiscovery
	enabled, err := q.Where(q.Enabled.Is(true)).Order(q.ID).Find()
	if err != nil {
		logger.Errorf("Failed to load the upstream discoveries: %v", err)
		return
	}

	now := r.now()
	var wg sync.WaitGroup
	for _, binding := range enabled {
		if binding.NextRunAt != nil && binding.NextRunAt.After(now) {
			continue
		}
		lock := r.lockOf(binding.ID)
		if !lock.TryLock() {
			continue
		}
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			defer lock.Unlock()
			if _, err := r.run(r.ctx, id); err != nil {
				logger.Warnf("Failed to refresh upstream discovery %d: %v", id, err)
			}
		}(binding.ID)
	}
	wg.Wait()
}

// Refresh resolves a binding now, due, disabled or not, since the person
// asked for it, and returns it with the outcome recorded.
func (r *Runner) Refresh(ctx context.Context, id uint64) (*model.UpstreamDiscovery, error) {
	lock := r.lockOf(id)
	lock.Lock()
	defer lock.Unlock()
	return r.run(ctx, id)
}

// Remove deletes the file of an upstream and reloads nginx. It refuses while
// the nginx configuration still includes the file or proxies to the
// upstream, which nginx would no longer accept.
func (r *Runner) Remove(upstreamName string) error {
	if !IsValidUpstreamName(upstreamName) {
		return nil
	}
	r.files.Lock()
	defer r.files.Unlock()

	if _, err := r.writer.Remove(r.Path(upstreamName)); err != nil {
		if testErr, ok := errors.AsType[*config.TestError](err); ok {
			return cosy.WrapErrorWithParams(plugin.ErrGeneratedFileIncluded, IncludePath(upstreamName), testErr.Err.Error())
		}
		return err
	}
	return nil
}

// outcome is the result of one run.
type outcome struct {
	status  string
	message string
	// targets is the number of servers now in the file, -1 when the file was
	// left as it was.
	targets int
	// ttl is what the provider asked for, 0 for no opinion.
	ttl int
}

// run resolves one binding and records the outcome.
func (r *Runner) run(ctx context.Context, id uint64) (*model.UpstreamDiscovery, error) {
	q := query.UpstreamDiscovery
	binding, err := q.Where(q.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}

	started := r.now()
	result := r.refresh(ctx, binding)
	if ctx.Err() != nil && result.status == model.RefreshStatusFailed {
		// The host is shutting down, the next start runs the binding again.
		return binding, nil
	}

	next := started.Add(nextInterval(binding.RefreshSeconds, result.ttl, result.status == model.RefreshStatusFailed))
	updates := []field.AssignExpr{
		q.LastRunAt.Value(started),
		q.NextRunAt.Value(next),
		q.LastStatus.Value(result.status),
		q.LastMessage.Value(result.message),
	}
	if result.targets >= 0 {
		updates = append(updates, q.TargetCount.Value(result.targets))
	}
	if _, err = q.Where(q.ID.Eq(id)).UpdateColumnSimple(updates...); err != nil {
		return nil, err
	}
	return q.Where(q.ID.Eq(id)).First()
}

// refresh resolves the service of a binding and writes its file.
func (r *Runner) refresh(ctx context.Context, binding *model.UpstreamDiscovery) outcome {
	if !IsValidUpstreamName(binding.UpstreamName) {
		return outcome{status: model.RefreshStatusFailed, message: fmt.Sprintf("upstream name %q is invalid", binding.UpstreamName), targets: -1}
	}

	result, err := Resolve(ctx, binding.Kind, binding.Config, binding.Service)
	if err != nil {
		return outcome{status: model.RefreshStatusFailed, message: err.Error(), targets: -1}
	}

	rendered, err := Render(binding.UpstreamName, result.Targets, binding.ExtraDirectives)
	if err != nil {
		message := err.Error()
		if rendered.Dropped > 0 {
			message = fmt.Sprintf("%s (%d invalid targets dropped)", message, rendered.Dropped)
		}
		return outcome{status: model.RefreshStatusFailed, message: message, targets: -1, ttl: result.TTLSeconds}
	}

	r.files.Lock()
	changed, err := r.writer.Apply(r.Path(binding.UpstreamName), rendered.Content)
	r.files.Unlock()
	if err != nil {
		return outcome{
			status:  model.RefreshStatusFailed,
			message: fmt.Sprintf("Resolved %d servers but could not apply them: %v", rendered.Written, err),
			targets: -1,
			ttl:     result.TTLSeconds,
		}
	}
	return outcome{
		status:  model.RefreshStatusOK,
		message: summary(rendered, changed),
		targets: rendered.Written,
		ttl:     result.TTLSeconds,
	}
}

// summary describes a successful run.
func summary(rendered Rendered, changed bool) string {
	parts := []string{fmt.Sprintf("%d servers written", rendered.Written)}
	if rendered.Dropped > 0 {
		parts = append(parts, fmt.Sprintf("%d invalid targets dropped", rendered.Dropped))
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

// nextInterval is how long a binding waits before its next run: its own
// interval, shortened by a shorter ttl of the provider, never below the
// minimum, and at most retryAfterFailure after a failure.
func nextInterval(refreshSeconds, ttlSeconds int, failed bool) time.Duration {
	seconds := refreshSeconds
	if seconds <= 0 {
		seconds = DefaultRefreshSeconds
	}
	if ttlSeconds > 0 && ttlSeconds < seconds {
		seconds = ttlSeconds
	}
	seconds = max(seconds, MinRefreshSeconds)
	interval := time.Duration(seconds) * time.Second
	if failed && interval > retryAfterFailure {
		interval = retryAfterFailure
	}
	return interval
}
