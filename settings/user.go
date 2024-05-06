package settings

type PredefinedUser struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

var PredefinedUserSettings = &PredefinedUser{}
