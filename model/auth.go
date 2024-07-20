package model

type Auth struct {
    Model

    Name     string `json:"name"`
    Password string `json:"-"`
    Status   bool   `json:"status" gorm:"default:1"`
}

type AuthToken struct {
    Token string `json:"token"`
}
