package plugin

// This file is the host side of the log.sink capability
// (nginx-ui-plugin-spec/spec/20-capabilities-logsink.md). The access log
// feed hands every batch of new lines to dispatchLogEntries, which offers it
// to the queue of every running log sink without blocking. One goroutine per
// plugin drains its queue into log.push client streams over the plugin's
// gRPC channel, one stream per batch.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	pluginv1 "github.com/0xJacky/Nginx-UI/internal/plugin/protocol/pb"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	// logSinkQueueSize bounds the entries waiting for one plugin (spec
	// LOGSINK-9). A plugin that cannot keep up loses lines instead of
	// holding anything back.
	logSinkQueueSize = 8192
	// logSinkStreamTimeout bounds one stream, starting an on_demand plugin
	// included (spec LOGSINK-10).
	logSinkStreamTimeout = 30 * time.Second
	// logSinkStopGrace is how long stopping a sink waits for the stream in
	// flight before it is cancelled.
	logSinkStopGrace = 5 * time.Second
	// Backoff after failed streams, doubled after every failure in a row.
	logSinkMinBackoff = time.Second
	logSinkMaxBackoff = 30 * time.Second
)

// errLogSinkNeedsGRPC is the last error of a log.sink plugin that does not
// serve gRPC, since log.push has no stdio form (spec LOGSINK-11).
var errLogSinkNeedsGRPC = errors.New("log.sink requires the grpc transport")

// LogFeed connects the log.sink capability to the access log pipeline of the
// host.
type LogFeed interface {
	// Subscribe delivers every batch of new access log lines to deliver until
	// the returned function is called. deliver must not block, and entries
	// is shared and must not be modified.
	Subscribe(deliver func(entries []protocol.LogSinkPushParams)) (unsubscribe func())
}

// logSinkCounters survive restarts of the plugin process.
type logSinkCounters struct {
	// streamed counts the entries the plugin accepted.
	streamed atomic.Int64
	// rejected counts the entries the plugin received and discarded.
	rejected atomic.Int64
	// dropped counts the entries that never reached the plugin.
	dropped atomic.Int64
}

// logSinkConfig is the effective log_sink block of a manifest.
type logSinkConfig struct {
	batchSize     int
	flushInterval time.Duration
	// formats lists the accepted LogEntry.Format values, empty means all.
	formats []string
}

func logSinkConfigOf(block *protocol.ManifestLogSink) logSinkConfig {
	cfg := logSinkConfig{
		batchSize:     protocol.DefaultLogSinkBatchSize,
		flushInterval: protocol.DefaultLogSinkFlushIntervalMS * time.Millisecond,
	}
	if block == nil {
		return cfg
	}
	if block.BatchSize > 0 {
		cfg.batchSize = min(block.BatchSize, protocol.MaxLogSinkBatchSize)
	}
	if block.FlushIntervalMS > 0 {
		cfg.flushInterval = time.Duration(max(block.FlushIntervalMS, protocol.MinLogSinkFlushIntervalMS)) * time.Millisecond
	}
	cfg.formats = slices.Clone(block.Formats)
	return cfg
}

func (c logSinkConfig) wants(format string) bool {
	return len(c.formats) == 0 || slices.Contains(c.formats, format)
}

// logStreamOpener opens one log.push stream. release is called once the
// stream is done and may be non-nil even when err is not.
type logStreamOpener func(ctx context.Context) (stream *grpcbridge.ClientStream, release func(), err error)

// logSink is the queue and the streamer of one plugin.
type logSink struct {
	pluginID string
	cfg      logSinkConfig
	counters *logSinkCounters
	open     logStreamOpener
	// report records a failure as the last error of the plugin, nil clears
	// it again.
	report func(err error)
	log    *zap.SugaredLogger

	queue   chan *protocol.LogSinkPushParams
	stopped atomic.Bool
	stopCh  chan struct{}
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	once    sync.Once

	// Timing, shortened by the tests.
	streamTimeout time.Duration
	minBackoff    time.Duration
	maxBackoff    time.Duration
	stopGrace     time.Duration
}

func newLogSink(pluginID string, cfg logSinkConfig, counters *logSinkCounters, open logStreamOpener,
	report func(error), log *zap.SugaredLogger) *logSink {
	ctx, cancel := context.WithCancel(context.Background())
	return &logSink{
		pluginID:      pluginID,
		cfg:           cfg,
		counters:      counters,
		open:          open,
		report:        report,
		log:           log,
		queue:         make(chan *protocol.LogSinkPushParams, logSinkQueueSize),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
		streamTimeout: logSinkStreamTimeout,
		minBackoff:    logSinkMinBackoff,
		maxBackoff:    logSinkMaxBackoff,
		stopGrace:     logSinkStopGrace,
	}
}

func (s *logSink) start() { go s.run() }

// offer queues the entries the plugin wants without blocking; what does not
// fit is dropped and counted.
func (s *logSink) offer(entries []protocol.LogSinkPushParams) {
	for i := range entries {
		entry := &entries[i]
		if !s.cfg.wants(entry.Entry.Format) {
			continue
		}
		if s.stopped.Load() {
			s.counters.dropped.Add(1)
			continue
		}
		select {
		case s.queue <- entry:
		default:
			s.counters.dropped.Add(1)
		}
	}
}

// stop ends the streamer. The stream in flight is closed and answered, or
// cancelled after the grace period; the rest of the queue is dropped.
func (s *logSink) stop() {
	s.once.Do(func() {
		s.stopped.Store(true)
		close(s.stopCh)
		grace := time.AfterFunc(s.stopGrace, s.cancel)
		<-s.done
		grace.Stop()
		s.cancel()
	})
}

func (s *logSink) run() {
	defer close(s.done)
	defer s.discard()

	backoff := time.Duration(0)
	failing := false
	for {
		// A closed stopCh and a non empty queue are both ready, and select
		// would pick one at random; stop always wins.
		if s.stopped.Load() {
			return
		}
		var first *protocol.LogSinkPushParams
		select {
		case <-s.stopCh:
			return
		case first = <-s.queue:
		}

		err := s.push(first)
		if err == nil {
			if failing {
				s.log.Infof("[plugin:%s] log.sink streams again", s.pluginID)
				if s.report != nil {
					s.report(nil)
				}
			}
			failing, backoff = false, 0
			continue
		}

		if errors.Is(err, errLogSinkNeedsGRPC) && s.report != nil {
			s.report(err)
		}
		if !failing {
			s.log.Warnf("[plugin:%s] log.sink stream failed, lines are dropped until it recovers: %v", s.pluginID, err)
		} else {
			s.log.Debugf("[plugin:%s] log.sink stream failed: %v", s.pluginID, err)
		}
		failing = true
		backoff = min(max(backoff*2, s.minBackoff), s.maxBackoff)

		timer := time.NewTimer(backoff)
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// discard drops what is left in the queue.
func (s *logSink) discard() {
	for {
		select {
		case <-s.queue:
			s.counters.dropped.Add(1)
		default:
			return
		}
	}
}

// push sends first and whatever follows it within the batch limits as one
// stream, and books the answer.
func (s *logSink) push(first *protocol.LogSinkPushParams) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.streamTimeout)
	defer cancel()

	stream, release, err := s.open(ctx)
	if release != nil {
		defer release()
	}
	if err != nil {
		s.counters.dropped.Add(1)
		return err
	}

	sent := 0
	send := func(entry *protocol.LogSinkPushParams) bool {
		in, err := marshalLogSinkEntry(entry)
		if err == nil {
			err = stream.Send(in)
		}
		if err != nil {
			// The peer ended the stream, CloseAndRecv says why.
			s.counters.dropped.Add(1)
			return false
		}
		sent++
		return true
	}

	if send(first) && sent < s.cfg.batchSize {
		flush := time.NewTimer(s.cfg.flushInterval)
		defer flush.Stop()
	fill:
		for sent < s.cfg.batchSize {
			select {
			case entry := <-s.queue:
				if !send(entry) {
					break fill
				}
			case <-flush.C:
				break fill
			case <-s.stopCh:
				break fill
			case <-ctx.Done():
				break fill
			}
		}
	}

	out, err := stream.CloseAndRecv()
	if err != nil {
		s.counters.dropped.Add(int64(sent))
		return err
	}
	var res pluginv1.LogSinkPushResponse
	if err = proto.Unmarshal(out, &res); err != nil {
		s.counters.dropped.Add(int64(sent))
		return fmt.Errorf("decode the log.push answer: %w", err)
	}
	accepted := min(int(res.GetAccepted()), sent)
	s.counters.streamed.Add(int64(accepted))
	s.counters.rejected.Add(int64(sent - accepted))
	return nil
}

// marshalLogSinkEntry encodes one stream message with the generated types:
// this is the hot path, the JSON bridge of the unary calls is not needed.
func marshalLogSinkEntry(p *protocol.LogSinkPushParams) ([]byte, error) {
	e := p.Entry
	return proto.Marshal(&pluginv1.LogSinkPushRequest{
		LogPath: p.LogPath,
		Entry: &pluginv1.LogEntry{
			Timestamp:            e.Timestamp,
			RemoteAddr:           e.RemoteAddr,
			RequestMethod:        e.RequestMethod,
			RequestUri:           e.RequestURI,
			Protocol:             e.Protocol,
			Status:               int32(e.Status),
			BodyBytesSent:        float64(e.BodyBytesSent),
			Referer:              e.Referer,
			UserAgent:            e.UserAgent,
			UpstreamAddr:         e.UpstreamAddr,
			RequestTime:          e.RequestTime,
			UpstreamResponseTime: e.UpstreamResponseTime,
			Host:                 e.Host,
			Raw:                  e.Raw,
			Format:               e.Format,
		},
	})
}

// SetLogFeed connects the log.sink plugins to the access log feed. The feed
// runs only while at least one enabled log.sink plugin is up.
func (m *Manager) SetLogFeed(feed LogFeed) {
	m.logMu.Lock()
	defer m.logMu.Unlock()
	if m.logUnsubscribe != nil {
		m.logUnsubscribe()
		m.logUnsubscribe = nil
	}
	m.logFeed = feed
	if feed != nil && len(m.logSinkList) > 0 {
		m.logUnsubscribe = feed.Subscribe(m.dispatchLogEntries)
	}
}

// dispatchLogEntries offers one batch of lines to every running log sink. It
// runs on the feed goroutine and never blocks.
func (m *Manager) dispatchLogEntries(entries []protocol.LogSinkPushParams) {
	sinks := m.logSinks.Load()
	if sinks == nil {
		return
	}
	for _, sink := range *sinks {
		sink.offer(entries)
	}
}

// wantsLogSinkLocked reports whether the entry streams the access log: an
// enabled log.sink plugin whose approved permissions include log.read. The
// caller holds at least the read lock.
func wantsLogSinkLocked(item *entry) bool {
	// enabledWithCapabilityLocked already requires the approved set to match
	// the manifest, so the manifest permission is the granted one.
	return enabledWithCapabilityLocked(item, protocol.CapabilityLogSink) &&
		slices.Contains(item.manifest.Permissions, protocol.PermissionLogRead)
}

// startLogSink gives a log.sink plugin its queue and streamer and subscribes
// the feed for the first one.
func (m *Manager) startLogSink(item *entry, supervisor *Supervisor) {
	m.mu.Lock()
	if item.logSink != nil || !wantsLogSinkLocked(item) {
		m.mu.Unlock()
		return
	}
	id := item.id
	sink := newLogSink(id, logSinkConfigOf(item.manifest.LogSink), &item.logCounters,
		supervisor.openLogStream, func(err error) { m.reportLogSinkError(id, err) }, m.log)
	item.logSink = sink
	m.mu.Unlock()
	sink.start()

	m.logMu.Lock()
	defer m.logMu.Unlock()
	m.logSinkList = append(m.logSinkList, sink)
	m.publishLogSinksLocked()
	if m.logFeed != nil && m.logUnsubscribe == nil {
		m.logUnsubscribe = m.logFeed.Subscribe(m.dispatchLogEntries)
	}
}

// stopLogSink stops the streamer of a plugin and the feed with the last one.
func (m *Manager) stopLogSink(item *entry) {
	m.mu.Lock()
	sink := item.logSink
	item.logSink = nil
	m.mu.Unlock()
	if sink == nil {
		return
	}

	m.logMu.Lock()
	m.logSinkList = slices.DeleteFunc(m.logSinkList, func(s *logSink) bool { return s == sink })
	m.publishLogSinksLocked()
	var unsubscribe func()
	if len(m.logSinkList) == 0 {
		unsubscribe = m.logUnsubscribe
		m.logUnsubscribe = nil
	}
	m.logMu.Unlock()

	if unsubscribe != nil {
		unsubscribe()
	}
	sink.stop()
}

// publishLogSinksLocked refreshes the list dispatchLogEntries reads. The
// caller holds logMu.
func (m *Manager) publishLogSinksLocked() {
	sinks := slices.Clone(m.logSinkList)
	m.logSinks.Store(&sinks)
}

// reportLogSinkError records why a log sink cannot stream as the last error
// of the plugin, and clears it again once it streams.
func (m *Manager) reportLogSinkError(id string, cause error) {
	m.mu.Lock()
	item, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	message := ""
	if cause != nil {
		message = cause.Error()
	} else if item.lastErr != errLogSinkNeedsGRPC.Error() {
		// Only the error this file set is cleared.
		m.mu.Unlock()
		return
	}
	if item.lastErr == message {
		m.mu.Unlock()
		return
	}
	item.lastErr = message
	var rowID uint64
	if item.row != nil {
		item.row.LastError = message
		rowID = item.row.ID
	}
	m.mu.Unlock()
	m.persistLastError(rowID, message)
}

// openLogStream opens a log.push stream to the plugin, starting an on_demand
// one. The stream exists on gRPC only.
func (s *Supervisor) openLogStream(ctx context.Context) (*grpcbridge.ClientStream, func(), error) {
	_, release, err := s.Acquire(ctx)
	if err != nil {
		return nil, release, err
	}
	client, err := s.grpcClient()
	if err != nil {
		return nil, release, err
	}
	stream, err := client.OpenStream(ctx, protocol.MethodLogPush)
	return stream, release, err
}

// grpcClient returns the gRPC channel of the running plugin.
func (s *Supervisor) grpcClient() (*grpcbridge.Client, error) {
	s.mu.Lock()
	p := s.proc
	running := p != nil && s.state == StateRunning
	s.mu.Unlock()
	if !running {
		return nil, ErrPluginNotRunning
	}
	if !slices.Contains(p.initResult.Transports, protocol.TransportGRPC) {
		return nil, errLogSinkNeedsGRPC
	}
	client := p.grpc.client.Load()
	if client == nil {
		return nil, fmt.Errorf("%w: the gRPC channel of the plugin is down", grpcbridge.ErrUnavailable)
	}
	return client, nil
}
