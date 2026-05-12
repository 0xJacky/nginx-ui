package validation

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/go-acme/lego/v5/certcrypto"
	val "github.com/go-playground/validator/v10"
)

func autoCertKeyType(fl val.FieldLevel) bool {
	return helper.IsValidKeyType(certcrypto.KeyType(fl.Field().String()))
}
