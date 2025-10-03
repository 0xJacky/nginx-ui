package settings

type NginxLog struct {
	IndexingEnabled bool   `json:"indexing_enabled"`
	IndexPath       string `json:"index_path"`
}

var NginxLogSettings = &NginxLog{}