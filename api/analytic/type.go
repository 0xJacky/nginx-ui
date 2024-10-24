package analytic

import (
	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/net"
)

type CPUStat struct {
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
	Total  float64 `json:"total"`
}

type Stat struct {
	Uptime  uint64             `json:"uptime"`
	LoadAvg *load.AvgStat      `json:"loadavg"`
	CPU     CPUStat            `json:"cpu"`
	Memory  analytic.MemStat   `json:"memory"`
	Disk    analytic.DiskStat  `json:"disk"`
	Network net.IOCountersStat `json:"network"`
}

type CPURecords struct {
	Info  []cpu.InfoStat            `json:"info"`
	User  []analytic.Usage[float64] `json:"user"`
	Total []analytic.Usage[float64] `json:"total"`
}

type NetworkRecords struct {
	Init      net.IOCountersStat       `json:"init"`
	BytesRecv []analytic.Usage[uint64] `json:"bytesRecv"`
	BytesSent []analytic.Usage[uint64] `json:"bytesSent"`
}

type DiskIORecords struct {
	Writes []analytic.Usage[uint64] `json:"writes"`
	Reads  []analytic.Usage[uint64] `json:"reads"`
}

type InitResp struct {
	Host    *host.InfoStat    `json:"host"`
	CPU     CPURecords        `json:"cpu"`
	Network NetworkRecords    `json:"network"`
	DiskIO  DiskIORecords     `json:"disk_io"`
	Memory  analytic.MemStat  `json:"memory"`
	Disk    analytic.DiskStat `json:"disk"`
	LoadAvg *load.AvgStat     `json:"loadavg"`
}
