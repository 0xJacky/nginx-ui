package model

import (
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
)

type DnsCredential struct {
	Model
	Name     string      `json:"name"`
	Config   *dns.Config `json:"config,omitempty" gorm:"serializer:json"`
	Provider string      `json:"provider"`
}
