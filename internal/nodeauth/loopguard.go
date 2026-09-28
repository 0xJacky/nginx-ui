package nodeauth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// Receivers do not fan out a write marked with Replicated-From, which is what
// keeps two nodes that sync to each other from bouncing it forever. Peers
// running an older release ignore that marker, though, so the transport also
// breaks a cycle on its own: each lap makes the sender repeat a byte-identical
// write to the same node, which normal operation never does at this rate.
const (
	loopGuardWindow    = time.Minute
	loopGuardThreshold = 5
	loopGuardSweepSize = 256
)

// ErrReplicationLoop is returned instead of sending a write that is part of a
// replication cycle between nodes.
var ErrReplicationLoop = errors.New("replication loop detected")

type loopGuard struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	history map[string][]time.Time
}

func newLoopGuard(window time.Duration, limit int) *loopGuard {
	return &loopGuard{window: window, limit: limit, history: make(map[string][]time.Time)}
}

var defaultLoopGuard = newLoopGuard(loopGuardWindow, loopGuardThreshold)

// allow records a send of key at now and reports whether it may go out. A
// rejected send is not recorded, so the key frees itself once the cycle has
// stopped for a full window.
func (guard *loopGuard) allow(key string, now time.Time) bool {
	guard.mu.Lock()
	defer guard.mu.Unlock()

	cutoff := now.Add(-guard.window)
	if len(guard.history) > loopGuardSweepSize {
		for historyKey, sends := range guard.history {
			if !sends[len(sends)-1].After(cutoff) {
				delete(guard.history, historyKey)
			}
		}
	}

	sends := guard.history[key]
	kept := sends[:0]
	for _, sent := range sends {
		if sent.After(cutoff) {
			kept = append(kept, sent)
		}
	}
	if len(kept) >= guard.limit {
		guard.history[key] = kept
		return false
	}
	guard.history[key] = append(kept, now)
	return true
}

// replicationTransport marks and guards the requests a node sends on its own
// behalf. The user-facing node proxy is deliberately left out: a change a
// person makes through the proxy must still reach that node's sync targets,
// and repeating an action by hand is not a cycle.
type replicationTransport struct {
	next   http.RoundTripper
	nodeID uint64
	guard  *loopGuard
}

func newReplicationTransport(nodeID uint64, next http.RoundTripper) http.RoundTripper {
	return &replicationTransport{next: next, nodeID: nodeID, guard: defaultLoopGuard}
}

func (transport *replicationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	markReplicated(request)

	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return transport.next.RoundTrip(request)
	}

	bodyDigest, err := requestBodyDigest(request)
	if err != nil {
		closeRequestBody(request)
		return nil, err
	}

	key := fmt.Sprintf("%d %s %s %s", transport.nodeID, request.Method, request.URL.String(), bodyDigest)
	if !transport.guard.allow(key, time.Now()) {
		closeRequestBody(request)
		logger.Errorf("Dropping node request that repeats inside a replication cycle: node_id=%d method=%s path=%s",
			transport.nodeID, request.Method, request.URL.Path)
		return nil, fmt.Errorf("%w: the same %s %s was sent to this node %d times within %s; check for nodes that sync to each other",
			ErrReplicationLoop, request.Method, request.URL.Path, transport.guard.limit, transport.guard.window)
	}

	return transport.next.RoundTrip(request)
}

// requestBodyDigest hashes the body without consuming it for the next
// transport. Bodies without GetBody are staged to a temporary file first.
func requestBodyDigest(request *http.Request) (string, error) {
	if request.Body == nil || request.Body == http.NoBody {
		return "", nil
	}
	if request.GetBody == nil {
		return stageRequestBody(request)
	}

	body, err := request.GetBody()
	if err != nil {
		return "", err
	}
	defer body.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, body); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func closeRequestBody(request *http.Request) {
	if request.Body != nil {
		_ = request.Body.Close()
	}
}
