package model

// Access rule types. A reference rule expands the rules of another access
// list in place.
const (
	AccessRuleAllow = "allow"
	AccessRuleDeny  = "deny"
	AccessRuleRef   = "ref"
)

// Access list fallbacks decide what happens to a request that no rule
// matched.
const (
	AccessFallbackDeny  = "deny"
	AccessFallbackAllow = "allow"
)

// AccessList is a named, ordered set of allow and deny rules. Nginx UI renders
// every list to its own file below the Nginx configuration directory, and
// sites, streams and locations use it through a single include directive.
type AccessList struct {
	Model
	Name string `json:"name" gorm:"not null"`
	// Slug names the rendered file. It is fixed at creation so renaming a list
	// never breaks the includes that point at it.
	Slug     string       `json:"slug" gorm:"uniqueIndex;not null"`
	Rules    []AccessRule `json:"rules" gorm:"serializer:json"`
	Fallback string       `json:"fallback" gorm:"default:deny"`
}

// AccessRule is one line of an access list.
type AccessRule struct {
	Type string `json:"type"`
	// Value holds the address for allow and deny rules.
	Value string `json:"value,omitempty"`
	// RefID holds the referenced list for ref rules.
	RefID uint64 `json:"ref_id,omitempty"`
	Note  string `json:"note,omitempty"`
}
