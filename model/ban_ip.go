package model

type BanIP struct {
	IP        string `json:"ip"`
	Attempts  int    `json:"attempts"`
	ExpiredAt int64  `json:"expired_at" gorm:"index"`
}
