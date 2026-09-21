package model

// Plugin sync policies.
const (
	PluginSyncPolicyManual = "manual"
	PluginSyncPolicyAuto   = "auto"
)

// Plugin persists the user facing state of an installed plugin. The manifest
// and files live on disk, the runtime state lives in the plugin manager.
type Plugin struct {
	Model
	PluginID string `json:"plugin_id" gorm:"uniqueIndex"`
	Version  string `json:"version"`
	Enabled  bool   `json:"enabled"`
	// Settings holds the values of the manifest settings_schema, encrypted at rest.
	Settings map[string]any `json:"settings" gorm:"serializer:json[aes]"`
	// ApprovedPermissionsHash is the hash of the permission set the user approved.
	// A different hash after an upgrade blocks enabling until re-approved.
	ApprovedPermissionsHash string `json:"approved_permissions_hash"`
	LastError               string `json:"last_error"`
	// Cluster sync, same pattern as Site.SyncNodeIDs.
	SyncPolicy   string   `json:"sync_policy"`
	SyncNodeIDs  []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	SyncSettings bool     `json:"sync_settings"`
}

// PluginKV is the private key value store of a plugin.
type PluginKV struct {
	Model
	PluginID string `json:"plugin_id" gorm:"uniqueIndex:idx_plugin_kv"`
	Key      string `json:"key" gorm:"uniqueIndex:idx_plugin_kv"`
	Value    []byte `json:"value"`
}
