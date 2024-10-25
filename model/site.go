package model

type Site struct {
	Model
	Path           string        `json:"path"`
	Advanced       bool          `json:"advanced"`
	SiteCategoryID uint64        `json:"site_category_id"`
	SiteCategory   *SiteCategory `json:"site_category,omitempty"`
}
