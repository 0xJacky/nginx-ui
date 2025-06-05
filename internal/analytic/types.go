package analytic

type MemStat struct {
	Total       string  `json:"total"`
	Used        string  `json:"used"`
	Cached      string  `json:"cached"`
	Free        string  `json:"free"`
	SwapUsed    string  `json:"swap_used"`
	SwapTotal   string  `json:"swap_total"`
	SwapCached  string  `json:"swap_cached"`
	SwapPercent float64 `json:"swap_percent"`
	Pressure    float64 `json:"pressure"`
}

type PartitionStat struct {
	Mountpoint string  `json:"mountpoint"`
	Device     string  `json:"device"`
	Fstype     string  `json:"fstype"`
	Total      string  `json:"total"`
	Used       string  `json:"used"`
	Free       string  `json:"free"`
	Percentage float64 `json:"percentage"`
}

type DiskStat struct {
	Total      string          `json:"total"`
	Used       string          `json:"used"`
	Percentage float64         `json:"percentage"`
	Writes     Usage[uint64]   `json:"writes"`
	Reads      Usage[uint64]   `json:"reads"`
	Partitions []PartitionStat `json:"partitions"`
}
