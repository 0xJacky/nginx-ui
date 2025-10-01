package settings

type NginxLog struct {
	AdvancedIndexingEnabled bool   `json:"advanced_indexing_enabled"`
	IndexPath               string `json:"index_path"`
}

var NginxLogSettings = &NginxLog{}