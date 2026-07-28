package settings

type Node struct {
	Name                 string `json:"name" binding:"omitempty,safety_text"`
	Secret               string `json:"secret" protected:"true" sensitive:"true"`
	InstanceID           string `json:"instance_id" protected:"true"`
	LegacyAuthEnabled    bool   `json:"legacy_auth_enabled"`
	LegacyMCPAuthEnabled bool   `json:"legacy_mcp_auth_enabled"`
	SkipInstallation     bool   `json:"skip_installation" protected:"true"`
	Demo                 bool   `json:"demo" protected:"true"`
	ICPNumber            string `json:"icp_number" binding:"omitempty,safety_text"`
	PublicSecurityNumber string `json:"public_security_number" binding:"omitempty,safety_text"`
}

var NodeSettings = &Node{}
