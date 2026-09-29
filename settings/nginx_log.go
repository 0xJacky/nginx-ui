package settings

// NginxLog keeps the two switches of the advanced log indexing that used to
// live in the host. Log analytics is a plugin now: both values are read only
// and only tell the host whether a node still has to hand its indexes over.
type NginxLog struct {
	IndexingEnabled bool   `json:"indexing_enabled" protected:"true"`
	IndexPath       string `json:"index_path" protected:"true"`
}

var NginxLogSettings = &NginxLog{}
