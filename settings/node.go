package settings

type Node struct {
	Name             string `json:"name" binding:"omitempty,safety_text"`
	Secret           string `json:"secret" protected:"true"`
	SkipInstallation bool   `json:"skip_installation" protected:"true"`
	Demo             bool   `json:"demo" protected:"true"`
}

var NodeSettings = &Node{
	Name:             "",
	Secret:           "",
	SkipInstallation: false,
	Demo:             false,
}
