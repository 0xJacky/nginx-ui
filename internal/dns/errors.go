package dns

import (
	"github.com/uozi-tech/cosy"
)

// DNS domain errors
var (
	ErrDomainNotFound              = cosy.NewError(40401, "DNS domain not found")
	ErrDuplicateDomain             = cosy.NewError(50001, "Domain already exists for this credential")
	ErrInvalidDomain               = cosy.NewError(50002, "Invalid domain name format")
	ErrCredentialNotFound          = cosy.NewError(50003, "DNS credential not found")
	ErrInvalidCredential           = cosy.NewError(50004, "Invalid DNS credential configuration")
	ErrDDNSTargetRequired          = cosy.NewError(40010, "DDNS requires at least one record")
	ErrInvalidDDNSTargetType       = cosy.NewError(40011, "DDNS only supports A and AAAA records")
	ErrDDNSRecordNotFound          = cosy.NewError(40402, "DDNS target record not found")
	ErrInvalidDDNSInterval         = cosy.NewError(40012, "DDNS interval must be at least 60 seconds")
	ErrInvalidDDNSIPVersion        = cosy.NewError(40013, "DDNS IP version must be ipv4, ipv6, ipv4_ipv6, or ipv6_ipv4")
	ErrDDNSIPVersionRecordMismatch = cosy.NewError(40014, "DDNS record type does not match the selected IP version")
	ErrDDNSIPUnavailable           = cosy.NewError(50005, "DDNS cannot detect a public IP to create records")
)
