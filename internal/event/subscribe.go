package event

import (
	"sync"
	"sync/atomic"
)

// Subscriber receives every published event. Implementations must not block.
type Subscriber func(Event)

var (
	subscribersMu sync.RWMutex
	subscribers   = map[uint64]Subscriber{}
	subscriberSeq atomic.Uint64
)

// Subscribe registers fn for every event published on the global bus and
// returns a function that removes the subscription.
func Subscribe(fn Subscriber) (unsubscribe func()) {
	id := subscriberSeq.Add(1)
	subscribersMu.Lock()
	subscribers[id] = fn
	subscribersMu.Unlock()
	return func() {
		subscribersMu.Lock()
		delete(subscribers, id)
		subscribersMu.Unlock()
	}
}

func dispatchToSubscribers(event Event) {
	subscribersMu.RLock()
	list := make([]Subscriber, 0, len(subscribers))
	for _, fn := range subscribers {
		list = append(list, fn)
	}
	subscribersMu.RUnlock()
	for _, fn := range list {
		fn(event)
	}
}
