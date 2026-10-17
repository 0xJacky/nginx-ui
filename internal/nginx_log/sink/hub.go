// Package sink hands the nginx access log lines to subscribers while nginx
// writes them. It feeds the log.sink plugin capability
// (docs/plugin/capabilities/log-sink.md).
//
// The rest of the log pipeline parses lines only while it indexes, in batches
// every few minutes and only with advanced indexing on, and it reads rotated
// files again from their start. None of that suits a stream of new lines, so
// the hub runs a small tail of its own: one goroutine that polls the access
// logs the host knows, and only while at least one subscriber is registered.
// Without a subscriber nothing runs and no file is open.
package sink

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Entry is one access log line.
type Entry struct {
	// LogPath is the access log the line was read from.
	LogPath string
	// Time is the time of the request, or the time the line was read when it
	// could not be parsed.
	Time time.Time
	// Parsed reports whether the line was read as the combined format. When
	// it is false only LogPath, Time and Raw are set.
	Parsed bool

	RemoteAddr    string
	Method        string
	URI           string
	Protocol      string
	Status        int
	BodyBytesSent int64
	Referer       string
	UserAgent     string
	// UpstreamAddr and Host stay empty: the combined format carries neither.
	UpstreamAddr string
	Host         string
	// RequestTime and UpstreamResponseTime are in seconds, 0 when the line
	// does not carry them.
	RequestTime          float64
	UpstreamResponseTime float64

	// Raw is the line without its line break.
	Raw string
}

// Subscriber receives the lines of every poll.
type Subscriber interface {
	// Deliver runs on the feeder goroutine and must not block. entries is
	// shared between the subscribers and must not be modified.
	Deliver(entries []Entry)
}

// SubscriberFunc adapts a function to Subscriber.
type SubscriberFunc func(entries []Entry)

// Deliver implements Subscriber.
func (f SubscriberFunc) Deliver(entries []Entry) { f(entries) }

// Source tells the feeder which files to follow.
type Source struct {
	// Paths lists the access logs the host knows.
	Paths func() []string
	// Allowed reports whether a person allowed the host to read a log, the
	// whitelist of the log viewer. Nil allows every path.
	Allowed func(path string) bool
}

// Config tunes the feeder. Zero values take the defaults.
type Config struct {
	// PollInterval is how often the followed files are read.
	PollInterval time.Duration
	// RefreshInterval is how often the list of access logs is read again.
	RefreshInterval time.Duration
	// MaxReadPerPoll bounds the bytes read from one file in one poll.
	MaxReadPerPoll int64
	// MaxLineLength bounds one line; a longer one is dropped.
	MaxLineLength int
}

// Feeder defaults.
const (
	DefaultPollInterval    = time.Second
	DefaultRefreshInterval = 10 * time.Second
	DefaultMaxReadPerPoll  = 4 << 20
	DefaultMaxLineLength   = 16 << 10
)

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultPollInterval
	}
	if c.RefreshInterval <= 0 {
		c.RefreshInterval = DefaultRefreshInterval
	}
	if c.MaxReadPerPoll <= 0 {
		c.MaxReadPerPoll = DefaultMaxReadPerPoll
	}
	if c.MaxLineLength <= 0 {
		c.MaxLineLength = DefaultMaxLineLength
	}
	return c
}

// Hub fans the lines out to its subscribers and runs the feeder while it has
// any.
type Hub struct {
	cfg Config

	// mu guards source, subs, next and the feeder handles.
	mu     sync.Mutex
	source Source
	subs   map[uint64]Subscriber
	next   uint64
	cancel context.CancelFunc
	done   chan struct{}

	// snapshot is the subscriber list the feeder reads without the lock.
	snapshot atomic.Pointer[[]Subscriber]
}

// NewHub returns a hub whose feeder follows the files of source.
func NewHub(source Source, cfg Config) *Hub {
	return &Hub{cfg: cfg.withDefaults(), source: source, subs: map[uint64]Subscriber{}}
}

var defaultHub = NewHub(Source{}, Config{})

// Default is the process wide hub the log.sink capability subscribes to.
func Default() *Hub { return defaultHub }

// SetSource replaces the files the default hub follows. The nginx_log
// package installs the access logs of the host at start.
func SetSource(source Source) { defaultHub.SetSource(source) }

// Subscribe registers s with the default hub.
func Subscribe(s Subscriber) (unsubscribe func()) { return defaultHub.Subscribe(s) }

// SetSource replaces the files the feeder follows. A running feeder picks it
// up at its next refresh.
func (h *Hub) SetSource(source Source) {
	h.mu.Lock()
	h.source = source
	h.mu.Unlock()
}

func (h *Hub) currentSource() Source {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.source
}

// Active reports whether anybody subscribed, which is when the feeder runs.
func (h *Hub) Active() bool {
	subs := h.snapshot.Load()
	return subs != nil && len(*subs) > 0
}

// Subscribe registers s and starts the feeder for the first subscriber, which
// opens the logs before Subscribe returns: every line written afterwards is
// delivered. The returned function removes s again and stops the feeder with
// the last one; it waits until the feeder no longer calls s, so it must not
// be called from Deliver.
func (h *Hub) Subscribe(s Subscriber) (unsubscribe func()) {
	h.mu.Lock()
	id := h.next
	h.next++
	h.subs[id] = s
	h.publishLocked()
	if h.cancel == nil {
		t := newTailer(h.cfg)
		t.refresh(h.source)
		ctx, cancel := context.WithCancel(context.Background())
		h.cancel = cancel
		h.done = make(chan struct{})
		go h.run(ctx, h.done, t)
	}
	h.mu.Unlock()

	var once sync.Once
	return func() { once.Do(func() { h.unsubscribe(id) }) }
}

func (h *Hub) unsubscribe(id uint64) {
	h.mu.Lock()
	delete(h.subs, id)
	h.publishLocked()
	var done chan struct{}
	if len(h.subs) == 0 && h.cancel != nil {
		h.cancel()
		done = h.done
		h.cancel, h.done = nil, nil
	}
	h.mu.Unlock()

	if done != nil {
		<-done
	}
}

// publishLocked refreshes the snapshot in a stable order. The caller holds mu.
func (h *Hub) publishLocked() {
	ids := make([]uint64, 0, len(h.subs))
	for id := range h.subs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	subs := make([]Subscriber, 0, len(ids))
	for _, id := range ids {
		subs = append(subs, h.subs[id])
	}
	h.snapshot.Store(&subs)
}

// deliver hands one batch to every subscriber.
func (h *Hub) deliver(entries []Entry) {
	if len(entries) == 0 {
		return
	}
	subs := h.snapshot.Load()
	if subs == nil {
		return
	}
	for _, s := range *subs {
		s.Deliver(entries)
	}
}

// run is the feeder goroutine. t has its files open already.
func (h *Hub) run(ctx context.Context, done chan<- struct{}, t *tailer) {
	defer close(done)
	defer t.close()

	lastRefresh := time.Now()

	ticker := time.NewTicker(h.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if time.Since(lastRefresh) >= h.cfg.RefreshInterval {
			t.refresh(h.currentSource())
			lastRefresh = time.Now()
		}
		h.deliver(t.poll(ctx))
	}
}
