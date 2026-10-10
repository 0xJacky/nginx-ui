package user

import (
	"net"
	"sync"
	"time"
)

var mfaConnections = struct {
	sync.Mutex
	users map[uint64]map[*mfaConnection]struct{}
}{users: make(map[uint64]map[*mfaConnection]struct{})}

type mfaConnection struct {
	net.Conn
	userID uint64
	done   chan struct{}
	once   sync.Once
}

func (c *mfaConnection) Close() error {
	c.once.Do(func() {
		close(c.done)
		mfaConnections.Lock()
		delete(mfaConnections.users[c.userID], c)
		if len(mfaConnections.users[c.userID]) == 0 {
			delete(mfaConnections.users, c.userID)
		}
		mfaConnections.Unlock()
	})
	return c.Conn.Close()
}

// TrackMFAConnection also detects resets performed by a separate CLI process.
func TrackMFAConnection(conn net.Conn, id, version uint64) net.Conn {
	c := &mfaConnection{Conn: conn, userID: id, done: make(chan struct{})}
	mfaConnections.Lock()
	if mfaConnections.users[id] == nil {
		mfaConnections.users[id] = make(map[*mfaConnection]struct{})
	}
	mfaConnections.users[id][c] = struct{}{}
	mfaConnections.Unlock()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				current, err := MFAVersion(id)
				if err != nil || current != version {
					_ = c.Close()
					return
				}
			}
		}
	}()
	return c
}

func CloseMFAConnections(id uint64) {
	mfaConnections.Lock()
	connections := make([]*mfaConnection, 0, len(mfaConnections.users[id]))
	for conn := range mfaConnections.users[id] {
		connections = append(connections, conn)
	}
	mfaConnections.Unlock()
	for _, conn := range connections {
		_ = conn.Close()
	}
}
