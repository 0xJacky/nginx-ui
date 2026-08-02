package analytic

import (
	"context"
	"sync"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

type Usage[T uint64 | float64] struct {
	Time  time.Time `json:"x"`
	Usage T         `json:"y"`
}

var (
	CpuUserRecord   []Usage[float64]
	CpuTotalRecord  []Usage[float64]
	NetRecvRecord   []Usage[uint64]
	NetSentRecord   []Usage[uint64]
	DiskWriteRecord []Usage[uint64]
	DiskReadRecord  []Usage[uint64]
	LastDiskWrites  uint64
	LastDiskReads   uint64
	LastNetSent     uint64
	LastNetRecv     uint64
	LastNetSampleAt time.Time
	initializeOnce  sync.Once
)

// Initialize prepares server analytics lazily so non-server commands do not
// collect host statistics during package initialization.
func Initialize() {
	initializeOnce.Do(func() {
		network, err := GetNetworkStat()
		if err != nil {
			logger.Error(err)
		}
		if network != nil {
			LastNetRecv = network.BytesRecv
			LastNetSent = network.BytesSent
		}

		LastDiskReads, LastDiskWrites = getTotalDiskIO()

		now := time.Now()
		LastNetSampleAt = now
		// Initialize record slices.
		for i := 100; i > 0; i-- {
			uf := Usage[float64]{Time: now.Add(time.Duration(-i) * time.Second), Usage: 0}
			CpuUserRecord = append(CpuUserRecord, uf)
			CpuTotalRecord = append(CpuTotalRecord, uf)
			u := Usage[uint64]{Time: now.Add(time.Duration(-i) * time.Second), Usage: 0}
			NetRecvRecord = append(NetRecvRecord, u)
			NetSentRecord = append(NetSentRecord, u)
			DiskWriteRecord = append(DiskWriteRecord, u)
			DiskReadRecord = append(DiskReadRecord, u)
		}
	})
}

func RecordServerAnalytic(ctx context.Context) {
	Initialize()
	logger.Info("RecordServerAnalytic Started")
	for {
		select {
		case <-ctx.Done():
			logger.Info("RecordServerAnalytic Stopped")
			return
		case <-time.After(1 * time.Second):
			now := time.Now()
			recordCpu(now) // this func will spend more than 1 second.
			recordNetwork()
			recordDiskIO(now)
		}
	}
}
