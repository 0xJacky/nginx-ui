package demo

import (
	"hash/fnv"
	"time"
)

// bucketSeconds quantises time-varying values. Anything that should look alive
// (latency, counters, response times) is hashed against the bucket rather than
// a timestamp, so it steps on a boundary instead of jittering on every poll —
// and so the several nginx-ui instances sharing a demo container compute the
// same value at the same minute without coordinating.
const bucketSeconds = 300

// seed derives a stable 64-bit value from a domain and a key.
//
// Every fabricated value must be a pure function of its input. A shared
// math/rand source would make output depend on goroutine scheduling: the log
// parser runs a worker pool and memoises geo lookups per IP, so whichever
// answer landed first would be frozen in, differently on every run.
func seed(domain, key string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("nginxui-demo|"))
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(key))
	return h.Sum64()
}

// timeBucket is the current coarse time slice. Callers fold it into a seed to
// get a value that changes slowly but identically across processes.
func timeBucket(now time.Time) uint64 {
	return uint64(now.Unix() / bucketSeconds)
}

// pick chooses an element by hashed key. Returns the zero value for an empty
// table so callers do not have to guard.
func pick[T any](table []T, s uint64) T {
	var zero T
	if len(table) == 0 {
		return zero
	}
	return table[s%uint64(len(table))]
}

// rangeInt maps a seed into [lo, hi). Returns lo when the range is empty or
// inverted, so a bad table cannot panic a demo instance.
func rangeInt(s uint64, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + int(s%uint64(hi-lo))
}
