package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge/bridgetest"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// Fake plugin modes of the log.sink tests.
const (
	// pluginModeLogSink declares log.sink and serves the stream over gRPC.
	pluginModeLogSink = "logsink"
	// pluginModeLogSinkStdio declares log.sink but serves stdio only.
	pluginModeLogSinkStdio = "logsinkstdio"
)

// testMethodLogEntries asks the fake plugin how many entries it accepted.
const testMethodLogEntries = "test.log_entries"

// testPluginLogPush is the log.push stream of the fake plugin process. It
// accepts every entry but the ones with status 500.
func testPluginLogPush(_ context.Context, method string, next func() (json.RawMessage, error)) (any, error) {
	if method != protocol.MethodLogPush {
		return nil, &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method " + method}
	}
	var result protocol.LogSinkPushResult
	for {
		raw, err := next()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		var params protocol.LogSinkPushParams
		if err = json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		if params.Entry.Status == 500 {
			result.Rejected++
			continue
		}
		result.Accepted++
		testPluginGRPC.logEntries.Add(1)
	}
}

// streamRecorder is an in-process log.push server that records the size of
// every stream.
type streamRecorder struct {
	mu      sync.Mutex
	streams []int
	formats []string
	// block holds every stream open until it is closed.
	block chan struct{}
	// opened counts the streams that reached the recorder.
	opened atomic.Int32
}

func (r *streamRecorder) handle(ctx context.Context, method string, next func() (json.RawMessage, error)) (any, error) {
	r.opened.Add(1)
	if r.block != nil {
		select {
		case <-r.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return recordStream(r, next)
}

func recordStream(r *streamRecorder, next func() (json.RawMessage, error)) (any, error) {
	var result protocol.LogSinkPushResult
	count := 0
	for {
		raw, err := next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		var params protocol.LogSinkPushParams
		if err = json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		count++
		r.mu.Lock()
		r.formats = append(r.formats, params.Entry.Format)
		r.mu.Unlock()
		if params.Entry.Status == 500 {
			result.Rejected++
		} else {
			result.Accepted++
		}
	}
	r.mu.Lock()
	r.streams = append(r.streams, count)
	r.mu.Unlock()
	return result, nil
}

func (r *streamRecorder) sizes() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.streams...)
}

// bufconnOpener serves a stream handler in process and returns an opener
// that dials it.
func bufconnOpener(t *testing.T, handler bridgetest.StreamHandler) logStreamOpener {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := bridgetest.NewStreamServer(func(context.Context, string, json.RawMessage) (any, error) {
		return protocol.EmptyResult{}, nil
	}, handler)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := grpcbridge.DialTarget(ctx, "passthrough:///bufconn", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	return func(ctx context.Context) (*grpcbridge.ClientStream, func(), error) {
		stream, err := client.OpenStream(ctx, protocol.MethodLogPush)
		return stream, nil, err
	}
}

func testLogEntries(n int, format string, status int) []protocol.LogSinkPushParams {
	entries := make([]protocol.LogSinkPushParams, n)
	for i := range entries {
		entries[i] = protocol.LogSinkPushParams{
			LogPath: "/var/log/nginx/access.log",
			Entry:   protocol.LogEntry{Status: status, Raw: "line", Format: format, BodyBytesSent: 5 << 30},
		}
	}
	return entries
}

// newTestLogSink returns a sink with short timings that is stopped at the
// end of the test.
func newTestLogSink(t *testing.T, cfg logSinkConfig, open logStreamOpener, report func(error)) (*logSink, *logSinkCounters) {
	t.Helper()
	counters := &logSinkCounters{}
	sink := newLogSink("official.logs", cfg, counters, open, report, zap.NewNop().Sugar())
	sink.minBackoff = 5 * time.Millisecond
	sink.maxBackoff = 20 * time.Millisecond
	sink.stopGrace = 500 * time.Millisecond
	t.Cleanup(sink.stop)
	return sink, counters
}

func TestLogSinkConfigDefaultsAndBounds(t *testing.T) {
	cfg := logSinkConfigOf(nil)
	assert.Equal(t, protocol.DefaultLogSinkBatchSize, cfg.batchSize)
	assert.Equal(t, 500*time.Millisecond, cfg.flushInterval)
	assert.True(t, cfg.wants(protocol.LogFormatRaw))

	cfg = logSinkConfigOf(&protocol.ManifestLogSink{BatchSize: 100000, FlushIntervalMS: 10, Formats: []string{protocol.LogFormatCombined}})
	assert.Equal(t, protocol.MaxLogSinkBatchSize, cfg.batchSize)
	assert.Equal(t, 50*time.Millisecond, cfg.flushInterval)
	assert.True(t, cfg.wants(protocol.LogFormatCombined))
	assert.False(t, cfg.wants(protocol.LogFormatRaw))
}

func TestLogSinkStreamsBatches(t *testing.T) {
	recorder := &streamRecorder{}
	sink, counters := newTestLogSink(t, logSinkConfig{batchSize: 3, flushInterval: time.Second}, bufconnOpener(t, recorder.handle), nil)

	// Queue first, then start, so the batches are deterministic.
	sink.offer(testLogEntries(6, protocol.LogFormatCombined, 200))
	sink.offer(testLogEntries(1, protocol.LogFormatCombined, 500))
	sink.start()

	require.Eventually(t, func() bool { return counters.streamed.Load()+counters.rejected.Load() == 7 }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, []int{3, 3, 1}, recorder.sizes(), "a stream closes at batch_size, the last one at flush_interval")
	assert.EqualValues(t, 6, counters.streamed.Load())
	assert.EqualValues(t, 1, counters.rejected.Load())
	assert.Zero(t, counters.dropped.Load())
}

func TestLogSinkFlushesAfterTheInterval(t *testing.T) {
	recorder := &streamRecorder{}
	sink, counters := newTestLogSink(t, logSinkConfig{batchSize: 100, flushInterval: 30 * time.Millisecond}, bufconnOpener(t, recorder.handle), nil)
	sink.start()

	started := time.Now()
	sink.offer(testLogEntries(2, protocol.LogFormatCombined, 200))
	require.Eventually(t, func() bool { return counters.streamed.Load() == 2 }, 5*time.Second, time.Millisecond)
	assert.Less(t, time.Since(started), 2*time.Second)
	assert.Equal(t, []int{2}, recorder.sizes())
}

func TestLogSinkDropsWhatDoesNotFitAndFiltersFormats(t *testing.T) {
	recorder := &streamRecorder{}
	sink, counters := newTestLogSink(t, logSinkConfig{
		batchSize: protocol.MaxLogSinkBatchSize, flushInterval: 50 * time.Millisecond, formats: []string{protocol.LogFormatCombined},
	}, bufconnOpener(t, recorder.handle), nil)

	// Lines of an unwanted format are neither queued nor counted.
	sink.offer(testLogEntries(100, protocol.LogFormatRaw, 200))
	sink.offer(testLogEntries(logSinkQueueSize+25, protocol.LogFormatCombined, 200))
	assert.EqualValues(t, 25, counters.dropped.Load(), "the queue holds %d entries", logSinkQueueSize)

	sink.start()
	require.Eventually(t, func() bool { return counters.streamed.Load() == logSinkQueueSize }, 10*time.Second, 5*time.Millisecond)
	recorder.mu.Lock()
	for _, format := range recorder.formats {
		assert.Equal(t, protocol.LogFormatCombined, format)
	}
	recorder.mu.Unlock()
}

func TestLogSinkBacksOffAndReportsAMissingTransport(t *testing.T) {
	recorder := &streamRecorder{}
	open := bufconnOpener(t, recorder.handle)
	var failures atomic.Int32
	failing := func(ctx context.Context) (*grpcbridge.ClientStream, func(), error) {
		if failures.Add(1) <= 2 {
			return nil, func() {}, errLogSinkNeedsGRPC
		}
		return open(ctx)
	}
	var mu sync.Mutex
	var reports []error
	report := func(err error) {
		mu.Lock()
		reports = append(reports, err)
		mu.Unlock()
	}

	sink, counters := newTestLogSink(t, logSinkConfig{batchSize: 10, flushInterval: 20 * time.Millisecond}, failing, report)
	sink.start()
	for range 3 {
		sink.offer(testLogEntries(1, protocol.LogFormatCombined, 200))
		time.Sleep(15 * time.Millisecond)
	}

	require.Eventually(t, func() bool { return counters.streamed.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	assert.EqualValues(t, 2, counters.dropped.Load(), "the entry of every failed attempt is dropped")
	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(reports), 2)
	assert.ErrorIs(t, reports[0], errLogSinkNeedsGRPC)
	assert.NoError(t, reports[len(reports)-1], "a working stream clears the reported error")
}

func TestLogSinkStopFinishesTheStreamAndDropsTheRest(t *testing.T) {
	recorder := &streamRecorder{block: make(chan struct{})}
	sink, counters := newTestLogSink(t, logSinkConfig{batchSize: 5, flushInterval: time.Hour}, bufconnOpener(t, recorder.handle), nil)
	sink.offer(testLogEntries(12, protocol.LogFormatCombined, 200))
	sink.start()

	// The first stream is open and full; release it once stop was asked.
	require.Eventually(t, func() bool { return recorder.opened.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	stopped := make(chan struct{})
	go func() {
		sink.stop()
		close(stopped)
	}()
	require.Eventually(t, func() bool { return sink.stopped.Load() }, 5*time.Second, 5*time.Millisecond)
	close(recorder.block)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return")
	}

	assert.EqualValues(t, 5, counters.streamed.Load(), "the stream in flight is answered")
	assert.EqualValues(t, 7, counters.dropped.Load(), "the rest of the queue is dropped")

	sink.offer(testLogEntries(3, protocol.LogFormatCombined, 200))
	assert.EqualValues(t, 10, counters.dropped.Load(), "a stopped sink drops what it is offered")
}

func TestLogSinkStopCancelsAHungStream(t *testing.T) {
	recorder := &streamRecorder{block: make(chan struct{})}
	defer close(recorder.block)
	sink, counters := newTestLogSink(t, logSinkConfig{batchSize: 2, flushInterval: time.Hour}, bufconnOpener(t, recorder.handle), nil)
	sink.stopGrace = 50 * time.Millisecond
	sink.offer(testLogEntries(2, protocol.LogFormatCombined, 200))
	sink.start()
	time.Sleep(30 * time.Millisecond)

	started := time.Now()
	sink.stop()
	assert.Less(t, time.Since(started), 3*time.Second)
	assert.EqualValues(t, 2, counters.dropped.Load())
}

// fakeLogFeed hands the test the deliver function of the manager.
type fakeLogFeed struct {
	mu          sync.Mutex
	deliver     func([]protocol.LogSinkPushParams)
	subscribed  int
	unsubscribe int
}

func (f *fakeLogFeed) Subscribe(deliver func([]protocol.LogSinkPushParams)) func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliver = deliver
	f.subscribed++
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.deliver = nil
		f.unsubscribe++
	}
}

func (f *fakeLogFeed) push(entries []protocol.LogSinkPushParams) bool {
	f.mu.Lock()
	deliver := f.deliver
	f.mu.Unlock()
	if deliver == nil {
		return false
	}
	deliver(entries)
	return true
}

func (f *fakeLogFeed) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subscribed, f.unsubscribe
}

func logSinkManifest(id string) *protocol.Manifest {
	manifest := pluginManifest(id)
	manifest.Capabilities = []string{protocol.CapabilityLogSink}
	manifest.DNS01 = nil
	manifest.Permissions = []string{protocol.PermissionLogRead}
	manifest.LogSink = &protocol.ManifestLogSink{BatchSize: 2, FlushIntervalMS: 50}
	return manifest
}

func TestManagerStreamsTheAccessLogToLogSinkPlugins(t *testing.T) {
	usePluginProcesses(t, pluginModeLogSink)
	m := newTestManager(t)
	ctx := context.Background()
	feed := &fakeLogFeed{}
	m.SetLogFeed(feed)
	m.Start(ctx)

	subscribed, _ := feed.counts()
	assert.Zero(t, subscribed, "no log.sink plugin, no feed")

	info, err := m.Install(ctx, buildTestPackage(t, logSinkManifest("official.logs"), nil), InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	require.Equal(t, StatusRunning, info.Status, info.LastError)
	require.Eventually(t, func() bool { return transportOf(m, "official.logs") == protocol.TransportGRPC }, 5*time.Second, 10*time.Millisecond)

	subscribed, _ = feed.counts()
	assert.Equal(t, 1, subscribed)
	entries := append(testLogEntries(4, protocol.LogFormatCombined, 200), testLogEntries(1, protocol.LogFormatRaw, 500)...)
	require.True(t, feed.push(entries))

	require.Eventually(t, func() bool {
		info, err := m.Get("official.logs")
		return err == nil && info.StreamedLogEntries == 4 && info.RejectedLogEntries == 1
	}, 10*time.Second, 10*time.Millisecond)
	caller, err := supervisorOf(m, "official.logs").stdioClient()
	require.NoError(t, err)
	var got struct {
		Entries int64 `json:"entries"`
	}
	require.NoError(t, caller.Call(ctx, testMethodLogEntries, nil, &got))
	assert.EqualValues(t, 4, got.Entries)

	// Disabling the last log.sink plugin stops the feed.
	_, err = m.Disable(ctx, "official.logs")
	require.NoError(t, err)
	_, unsubscribed := feed.counts()
	assert.Equal(t, 1, unsubscribed)
	assert.False(t, feed.push(entries))

	info, err = m.Get("official.logs")
	require.NoError(t, err)
	assert.EqualValues(t, 4, info.StreamedLogEntries, "the counters survive a stop")
}

func TestManagerReportsALogSinkWithoutGRPC(t *testing.T) {
	usePluginProcesses(t, pluginModeLogSinkStdio)
	m := newTestManager(t)
	ctx := context.Background()
	feed := &fakeLogFeed{}
	m.Start(ctx)
	// The feed may be connected after the plugins came up.
	_, err := m.Install(ctx, buildTestPackage(t, logSinkManifest("official.stdio-logs"), nil), InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	m.SetLogFeed(feed)

	require.Eventually(t, func() bool {
		if !feed.push(testLogEntries(1, protocol.LogFormatCombined, 200)) {
			return false
		}
		info, err := m.Get("official.stdio-logs")
		return err == nil && info.LastError == errLogSinkNeedsGRPC.Error()
	}, 10*time.Second, 20*time.Millisecond)

	info, err := m.Get("official.stdio-logs")
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, info.Status)
	assert.Zero(t, info.StreamedLogEntries)
	assert.Positive(t, info.DroppedLogEntries)
}

func TestManagerSkipsLogSinkWithoutThePermission(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.LoadOffline(context.Background()))
	manifest := logSinkManifest("official.no-read")
	manifest.Permissions = nil
	item := m.newEntry(manifest.ID, manifest, &model.Plugin{PluginID: manifest.ID, Enabled: true})
	m.startLogSink(item, NewSupervisor(SupervisorConfig{PluginID: manifest.ID, Manifest: manifest}))
	assert.Nil(t, item.logSink, "log.read was not granted")

	manifest.Permissions = []string{protocol.PermissionLogRead}
	item = m.newEntry(manifest.ID, manifest, &model.Plugin{PluginID: manifest.ID, Enabled: true, ApprovedPermissionsHash: PermissionsHash(manifest)})
	m.startLogSink(item, NewSupervisor(SupervisorConfig{PluginID: manifest.ID, Manifest: manifest}))
	require.NotNil(t, item.logSink)
	m.stopLogSink(item)
	assert.Nil(t, item.logSink)
}

func TestConformanceLogSinkCases(t *testing.T) {
	var cases []CaseResult
	recordOn := func(transport string) recorder {
		return func(rule, name string, status CaseStatus, _ time.Duration, format string, _ ...any) {
			cases = append(cases, CaseResult{Rule: rule, Name: name, Transport: transport, Status: status, Message: format})
		}
	}
	statusOf := func(rule string) CaseStatus {
		for _, c := range cases {
			if c.Rule == rule {
				return c.Status
			}
		}
		return ""
	}

	notFound := staticCaller{err: &protocol.Error{Code: protocol.CodeMethodNotFound}}
	runLogSinkCases(t.Context(), notFound, nil, true, false, recordOn)
	assert.Equal(t, StatusPass, statusOf("LOGSINK-4"))
	assert.Equal(t, StatusSkip, statusOf("LOGSINK-5"), "a stdio only run skips the stream")

	cases = nil
	runLogSinkCases(t.Context(), staticCaller{}, nil, false, false, recordOn)
	assert.Equal(t, StatusFail, statusOf("LOGSINK-4"), "answering log.push on stdio fails")
	assert.Equal(t, StatusFail, statusOf("LOGSINK-5"), "a log.sink plugin without grpc fails")

	recorder := &streamRecorder{}
	open := bufconnOpener(t, recorder.handle)
	var got CaseStatus
	runLogPushStream(t.Context(), openerStreamer(open), func(_, _ string, status CaseStatus, _ time.Duration, _ string, _ ...any) { got = status })
	assert.Equal(t, StatusPass, got)
	assert.Equal(t, []int{3}, recorder.sizes())

	rejecting := bufconnOpener(t, func(context.Context, string, func() (json.RawMessage, error)) (any, error) {
		return protocol.LogSinkPushResult{Accepted: 2, Rejected: 1}, nil
	})
	runLogPushStream(t.Context(), openerStreamer(rejecting), func(_, _ string, status CaseStatus, _ time.Duration, _ string, _ ...any) { got = status })
	assert.Equal(t, StatusFail, got)
}

// openerStreamer adapts an opener to the client interface of the stream case.
type openerStreamer logStreamOpener

func (o openerStreamer) OpenStream(ctx context.Context, _ string) (*grpcbridge.ClientStream, error) {
	stream, _, err := o(ctx)
	return stream, err
}

// transportOf reports how capability calls reach a plugin.
func transportOf(m *Manager, id string) string {
	supervisor := supervisorOf(m, id)
	if supervisor == nil {
		return ""
	}
	return supervisor.Transport()
}
