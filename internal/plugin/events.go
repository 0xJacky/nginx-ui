package plugin

import (
	"context"
	"slices"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

const (
	// eventQueueSize bounds the per plugin delivery queue. A plugin that
	// cannot keep up loses events instead of stalling the event bus.
	eventQueueSize = 1024
	// eventDeliveryTimeout bounds writing one notification to a plugin.
	eventDeliveryTimeout = 10 * time.Second
)

// subscribeEvents hooks the manager onto the global event bus. The caller
// holds opMu.
func (m *Manager) subscribeEvents() {
	if m.unsubscribe != nil {
		return
	}
	m.unsubscribe = event.Subscribe(m.dispatchEvent)
}

// dispatchEvent enqueues one domain event for every running plugin that
// subscribed to it. Bus subscribers must not block, so a full queue drops the
// event and bumps the counter the API reports.
func (m *Manager) dispatchEvent(published event.Event) {
	eventType := string(published.Type)
	notification := protocol.EventNotification{
		Type: eventType,
		Data: published.Data,
		TS:   time.Now().Unix(),
	}

	m.mu.RLock()
	targets := make([]*entry, 0, len(m.entries))
	queues := make([]chan protocol.EventNotification, 0, len(m.entries))
	for _, item := range m.entries {
		if item.events == nil || item.manifest == nil || item.supervisor == nil {
			continue
		}
		if !slices.Contains(item.manifest.Events, eventType) {
			continue
		}
		if permission := eventPermission(eventType); permission != "" &&
			!slices.Contains(item.manifest.Permissions, permission) {
			continue
		}
		// An idle on_demand plugin is never started just to see an event.
		if item.state != StateRunning {
			continue
		}
		targets = append(targets, item)
		queues = append(queues, item.events)
	}
	m.mu.RUnlock()

	for i, queue := range queues {
		select {
		case queue <- notification:
		default:
			targets[i].dropped.Add(1)
		}
	}
}

// eventPermission returns the permission an event needs on top of being listed
// in the manifest, empty when it needs none. The approved set
// equals the manifest set for a plugin that runs, see wantsLogSinkLocked.
func eventPermission(eventType string) string {
	if eventType == protocol.EventLogPathsChanged {
		return protocol.PermissionLogFiles
	}
	return ""
}

// startEventPump gives one plugin its delivery queue and the goroutine that
// drains it.
func (m *Manager) startEventPump(item *entry) {
	m.mu.Lock()
	if item.events != nil {
		m.mu.Unlock()
		return
	}
	item.events = make(chan protocol.EventNotification, eventQueueSize)
	item.stop = make(chan struct{})
	item.drained = make(chan struct{})
	queue, stop, drained := item.events, item.stop, item.drained
	id := item.id
	m.mu.Unlock()

	go m.pumpEvents(id, queue, stop, drained)
}

// stopEventPump tears the delivery queue of one plugin down and waits for the
// goroutine to finish.
func (m *Manager) stopEventPump(item *entry) {
	m.mu.Lock()
	stop, drained := item.stop, item.drained
	item.events, item.stop, item.drained = nil, nil, nil
	m.mu.Unlock()

	if stop == nil {
		return
	}
	close(stop)
	<-drained
}

func (m *Manager) pumpEvents(id string, queue <-chan protocol.EventNotification, stop <-chan struct{}, drained chan<- struct{}) {
	defer close(drained)
	for {
		select {
		case <-stop:
			return
		case notification := <-queue:
			m.deliverEvent(id, notification)
		}
	}
}

// deliverEvent sends one notification to a running plugin. Events are fire and
// forget: a plugin that is down simply misses them.
func (m *Manager) deliverEvent(id string, notification protocol.EventNotification) {
	item, ok := m.lookup(id)
	if !ok {
		return
	}

	m.mu.RLock()
	supervisor := item.supervisor
	m.mu.RUnlock()
	if supervisor == nil {
		return
	}
	client, err := supervisor.Client()
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(m.context(), eventDeliveryTimeout)
	defer cancel()
	if err = client.Notify(ctx, protocol.MethodEventsOn, notification); err != nil {
		m.log.Debugf("[plugin:%s] deliver %s: %v", id, notification.Type, err)
	}
}
