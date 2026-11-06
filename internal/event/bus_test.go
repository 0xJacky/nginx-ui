package event

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitTimeout bounds every wait on a goroutine so a regression fails fast
// instead of hanging the test binary.
const waitTimeout = 5 * time.Second

type recordedMessage struct {
	event string
	data  interface{}
}

// recordingHub is a WebSocketHub that remembers every broadcast it receives.
type recordingHub struct {
	mu       sync.Mutex
	messages []recordedMessage
}

func (h *recordingHub) BroadcastMessage(event string, data interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, recordedMessage{event: event, data: data})
}

func (h *recordingHub) snapshot() []recordedMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]recordedMessage(nil), h.messages...)
}

// newTestBus builds an isolated bus so tests that shut it down do not affect
// the process-wide singleton.
func newTestBus(t *testing.T) *Bus {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Bus{ctx: ctx, cancel: cancel}
}

// installGlobalHub registers a recording hub on the global bus and removes it
// when the test ends.
func installGlobalHub(t *testing.T) *recordingHub {
	t.Helper()
	hub := &recordingHub{}
	SetWebSocketHub(hub)
	t.Cleanup(func() { SetWebSocketHub(nil) })
	return hub
}

func TestBusPublishWithoutHubIsNoop(t *testing.T) {
	bus := newTestBus(t)

	assert.NotPanics(t, func() {
		bus.Publish(Event{Type: TypeNotification, Data: "ignored"})
	})
}

func TestBusPublishForwardsTypeAndData(t *testing.T) {
	bus := newTestBus(t)
	hub := &recordingHub{}
	bus.SetWebSocketHub(hub)

	tests := []struct {
		name  string
		event Event
	}{
		{
			name:  "struct payload",
			event: Event{Type: TypeProcessingStatus, Data: ProcessingStatusData{IndexScanning: true}},
		},
		{
			name:  "string payload",
			event: Event{Type: TypeNotification, Data: "hello"},
		},
		{
			name:  "nil payload",
			event: Event{Type: TypeIndexScanning, Data: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(hub.snapshot())
			bus.Publish(tt.event)

			messages := hub.snapshot()
			require.Len(t, messages, before+1)
			got := messages[len(messages)-1]
			assert.Equal(t, string(tt.event.Type), got.event)
			assert.Equal(t, tt.event.Data, got.data)
		})
	}
}

func TestBusSetWebSocketHubReplacesPreviousHub(t *testing.T) {
	bus := newTestBus(t)
	first := &recordingHub{}
	second := &recordingHub{}

	bus.SetWebSocketHub(first)
	bus.Publish(Event{Type: TypeNotification, Data: 1})
	bus.SetWebSocketHub(second)
	bus.Publish(Event{Type: TypeNotification, Data: 2})
	bus.SetWebSocketHub(nil)
	bus.Publish(Event{Type: TypeNotification, Data: 3})

	assert.Equal(t, []recordedMessage{{event: string(TypeNotification), data: 1}}, first.snapshot())
	assert.Equal(t, []recordedMessage{{event: string(TypeNotification), data: 2}}, second.snapshot())
}

func TestBusShutdownCancelsContextAndDetachesHub(t *testing.T) {
	bus := newTestBus(t)
	hub := &recordingHub{}
	bus.SetWebSocketHub(hub)

	require.NoError(t, bus.Context().Err())

	bus.Shutdown()

	select {
	case <-bus.Context().Done():
	default:
		t.Fatal("bus context should be cancelled after Shutdown")
	}
	assert.ErrorIs(t, bus.Context().Err(), context.Canceled)

	bus.Publish(Event{Type: TypeNotification, Data: "after shutdown"})
	assert.Empty(t, hub.snapshot())
}

func TestBusConcurrentPublishDeliversEveryEvent(t *testing.T) {
	bus := newTestBus(t)
	hub := &recordingHub{}
	bus.SetWebSocketHub(hub)

	const publishers = 16
	const perPublisher = 50

	var wg sync.WaitGroup
	for i := range publishers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range perPublisher {
				bus.Publish(Event{Type: TypeNotification, Data: i*perPublisher + j})
			}
		}()
	}
	wg.Wait()

	messages := hub.snapshot()
	require.Len(t, messages, publishers*perPublisher)

	seen := make(map[int]bool, len(messages))
	for _, m := range messages {
		seen[m.data.(int)] = true
	}
	assert.Len(t, seen, publishers*perPublisher, "every published event should arrive exactly once")
}

func TestBusConcurrentPublishAndHubSwap(t *testing.T) {
	bus := newTestBus(t)
	hubs := []*recordingHub{{}, {}}

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range iterations {
			bus.SetWebSocketHub(hubs[i%2])
		}
	}()
	go func() {
		defer wg.Done()
		for i := range iterations {
			bus.Publish(Event{Type: TypeNotification, Data: i})
		}
	}()
	wg.Wait()

	// Publishes race with hub swaps, so which hub receives each event is
	// unspecified; the guarantee is that no event is delivered twice.
	total := len(hubs[0].snapshot()) + len(hubs[1].snapshot())
	assert.LessOrEqual(t, total, iterations)
}

func TestGetEventBusReturnsSingleton(t *testing.T) {
	first := GetEventBus()
	second := GetEventBus()

	require.NotNil(t, first)
	assert.Same(t, first, second)
	assert.NotNil(t, first.Context())
}

func TestPackagePublishUsesGlobalBus(t *testing.T) {
	hub := installGlobalHub(t)

	Publish(Event{Type: TypeProcessingStatus, Data: ProcessingStatusData{IndexScanning: true}})

	assert.Equal(t, []recordedMessage{{
		event: string(TypeProcessingStatus),
		data:  ProcessingStatusData{IndexScanning: true},
	}}, hub.snapshot())
}

func TestInitEventSystemShutsDownWhenContextIsCancelled(t *testing.T) {
	hub := installGlobalHub(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		InitEventSystem(ctx)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(waitTimeout):
		t.Fatal("InitEventSystem did not return after its context was cancelled")
	}

	select {
	case <-GetEventBus().Context().Done():
	default:
		t.Fatal("global bus context should be cancelled after the event system shuts down")
	}

	// Shutdown detaches the hub, so later publishes no longer reach it.
	Publish(Event{Type: TypeNotification, Data: "after shutdown"})
	assert.Empty(t, hub.snapshot())
}
