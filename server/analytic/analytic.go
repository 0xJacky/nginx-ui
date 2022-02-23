package analytic

import (
	"github.com/shirou/gopsutil/v3/net"
	"log"
	"time"
)

type usage struct {
	Time  time.Time   `json:"x"`
	Usage interface{} `json:"y"`
}

var (
	CpuUserRecord   []usage
	CpuTotalRecord  []usage
	NetRecvRecord   []usage
	NetSentRecord   []usage
	DiskWriteRecord []usage
	DiskReadRecord  []usage
	LastDiskWrites  uint64
	LastDiskReads   uint64
	LastNetSent     uint64
	LastNetRecv     uint64
)

func init() {
	network, _ := net.IOCounters(false)

	if len(network) > 0 {
		LastNetRecv = network[0].BytesRecv
		LastNetSent = network[0].BytesSent
	}

	LastDiskReads, LastDiskWrites = getTotalDiskIO()

	now := time.Now()
	// init record slices
	for i := 100; i > 0; i-- {
		u := usage{Time: now.Add(time.Duration(-i) * time.Second), Usage: 0}
		CpuUserRecord = append(CpuUserRecord, u)
		CpuTotalRecord = append(CpuTotalRecord, u)
		NetRecvRecord = append(NetRecvRecord, u)
		NetSentRecord = append(NetSentRecord, u)
		DiskWriteRecord = append(DiskWriteRecord, u)
		DiskReadRecord = append(DiskReadRecord, u)
	}
}

func RecordServerAnalytic() {
	log.Println("[Nginx UI] RecordServerAnalytic Started")
	for {
		now := time.Now()
		recordCpu(now) // this func will spend more than 1 second.
		recordNetwork(now)
		recordDiskIO(now)
	}
}
