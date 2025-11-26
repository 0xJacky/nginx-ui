package dns

import (
	"github.com/uozi-tech/cosy"
)

// DNS domain errors
var (
	ErrDomainNotFound     = cosy.NewError(40401, "DNS domain not found")
	ErrDuplicateDomain    = cosy.NewError(50001, "Domain already exists for this credential")
	ErrInvalidDomain      = cosy.NewError(50002, "Invalid domain name format")
	ErrCredentialNotFound = cosy.NewError(50003, "DNS credential not found")
	ErrInvalidCredential  = cosy.NewError(50004, "Invalid DNS credential configuration")
)
