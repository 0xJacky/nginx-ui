package settings

type NginxLog struct {
	AdvancedIndexingEnabled bool `json:"advanced_indexing_enabled"`
}

var NginxLogSettings = &NginxLog{}