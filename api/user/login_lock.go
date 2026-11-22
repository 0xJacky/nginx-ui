package user

import "sync"

// clientLock is one client's login lock. refs counts the requests holding
// or waiting for it, so the entry can be dropped once nobody needs it.
type clientLock struct {
	sync.Mutex
	refs int
}

// clientLocks hands out one lock per client key. An entry lives only while
// a request holds or waits for it, so the map grows with the number of
// logins in flight, not with the number of clients ever seen.
type clientLocks struct {
	mu    sync.Mutex
	locks map[string]*clientLock
}

// loginClientLocks serializes the login attempts of one client IP: the ban
// check, the credential verification and the recorded failure run as one
// step, so concurrent guesses from that client cannot all pass the check
// before the first failure is counted. Different clients never wait on each
// other.
var loginClientLocks = &clientLocks{locks: map[string]*clientLock{}}

// lock blocks until the lock for key is held and returns the function that
// releases it. The release function may be called more than once; only the
// first call has an effect.
func (l *clientLocks) lock(key string) (unlock func()) {
	l.mu.Lock()
	entry, ok := l.locks[key]
	if !ok {
		entry = &clientLock{}
		l.locks[key] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.Lock()
	return sync.OnceFunc(func() {
		entry.Unlock()
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	})
}
