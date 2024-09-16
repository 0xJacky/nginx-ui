package settings

type Auth struct {
	IPWhiteList         []string `json:"ip_white_list" binding:"omitempty,dive,ip" ini:",,allowshadow" protected:"true"`
	BanThresholdMinutes int      `json:"ban_threshold_minutes" binding:"min=1"`
	MaxAttempts         int      `json:"max_attempts" binding:"min=1"`
}

var AuthSettings = Auth{
	BanThresholdMinutes: 10,
	MaxAttempts:         10,
}
