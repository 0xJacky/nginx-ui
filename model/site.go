package model

type Site struct {
	Model
	Path           string        `json:"path" gorm:"uniqueIndex"`
	Advanced       bool          `json:"advanced"`
	SiteCategoryID uint64        `json:"site_category_id"`
	SiteCategory   *SiteCategory `json:"site_category,omitempty"`
	SyncNodeIDs    []uint64      `json:"sync_node_ids" gorm:"serializer:json"`
}
