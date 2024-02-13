package validation

import (
	"github.com/go-acme/lego/v4/certcrypto"
	val "github.com/go-playground/validator/v10"
)

func autoCertKeyType(fl val.FieldLevel) bool {
	switch certcrypto.KeyType(fl.Field().String()) {
	case certcrypto.RSA2048, certcrypto.RSA3072, certcrypto.RSA4096,
		certcrypto.EC256, certcrypto.EC384:
		return true
	}
	return false
}
