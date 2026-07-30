package model

// PostSyncActionType defines the type of action after synchronization
const (
	// PostSyncActionNone indicates no operation after sync
	PostSyncActionNone = "none"
	// PostSyncActionReloadNginx indicates reload Nginx after sync
	PostSyncActionReloadNginx = "reload_nginx"
)

// UpstreamTestType defines the type of upstream test
const (
	// UpstreamTestLocal indicates local upstream test
	UpstreamTestLocal = "local"
	// UpstreamTestRemote indicates remote upstream test
	UpstreamTestRemote = "remote"
	// UpstreamTestMirror indicates mirror upstream test
	UpstreamTestMirror = "mirror"
)

// DeployMode defines where configs should be deployed
const (
	// DeployModeLocal indicates deploy locally with optional remote sync
	DeployModeLocal = "local"
	// DeployModeRemote indicates deploy to remote nodes only
	DeployModeRemote = "remote"
)

// SyncStrategy defines when namespace content is replicated to the member nodes
const (
	// SyncStrategyManual only replicates on explicit user actions
	SyncStrategyManual = "manual"
	// SyncStrategyAuto periodically replicates the whole namespace
	SyncStrategyAuto = "auto"
)

// DefaultSyncIntervalMinutes is used when a namespace enables automatic sync
// without providing an interval.
const DefaultSyncIntervalMinutes = 30

// Namespace represents a group of environments that can be synced across nodes
type Namespace struct {
	Model
	Name                string   `json:"name"`
	SyncNodeIds         []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	OrderID             int      `json:"-" gorm:"default:0"`
	PostSyncAction      string   `json:"post_sync_action" gorm:"default:'reload_nginx'"`
	UpstreamTestType    string   `json:"upstream_test_type" gorm:"default:'local'"`
	DeployMode          string   `json:"deploy_mode" gorm:"default:'local'"`
	SyncStrategy        string   `json:"sync_strategy" gorm:"default:'manual'"`
	SyncIntervalMinutes int      `json:"sync_interval_minutes" gorm:"default:30"`
}

// IsRemoteDeploy reports whether the namespace content is only deployed to the
// member nodes. Local Nginx must never be validated or reloaded for it.
func (n *Namespace) IsRemoteDeploy() bool {
	return n != nil && n.DeployMode == DeployModeRemote
}

// IsAutoSync reports whether the namespace content should be replicated
// periodically without user interaction.
func (n *Namespace) IsAutoSync() bool {
	return n != nil && n.SyncStrategy == SyncStrategyAuto
}

// EffectiveSyncInterval returns the auto sync interval. Zero means "use the
// default", which is what the API accepts when the field is left unset.
func (n *Namespace) EffectiveSyncInterval() int {
	if n == nil || n.SyncIntervalMinutes <= 0 {
		return DefaultSyncIntervalMinutes
	}
	return n.SyncIntervalMinutes
}
