package validation

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	val "github.com/go-playground/validator/v10"
)

func isPublicKey(fl val.FieldLevel) bool {
	return cert.IsPublicKey(fl.Field().String())
}

func isPrivateKey(fl val.FieldLevel) bool {
	return cert.IsPrivateKey(fl.Field().String())
}

func isPublicKeyPath(fl val.FieldLevel) bool {
	return cert.IsPublicKeyPath(fl.Field().String())
}

func isPrivateKeyPath(fl val.FieldLevel) bool {
	return cert.IsPrivateKeyPath(fl.Field().String())
}
