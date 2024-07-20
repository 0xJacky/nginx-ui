package settings

type Auth struct {
	IPWhiteList         []string `ini:",,allowshadow"`
	BanThresholdMinutes int      `json:"ban_threshold_minutes" binding:"min=1"`
	MaxAttempts         int      `json:"max_attempts" binding:"min=1"`
}

var AuthSettings = Auth{}
