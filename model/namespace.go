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

// Namespace represents a group of environments that can be synced across nodes
type Namespace struct {
	Model
	Name             string   `json:"name"`
	SyncNodeIds      []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	OrderID          int      `json:"-" gorm:"default:0"`
	PostSyncAction   string   `json:"post_sync_action" gorm:"default:'reload_nginx'"`
	UpstreamTestType string   `json:"upstream_test_type" gorm:"default:'local'"`
	DeployMode       string   `json:"deploy_mode" gorm:"default:'local'"`
}
