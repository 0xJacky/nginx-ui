package validation

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	val "github.com/go-playground/validator/v10"
)

func isCertificate(fl val.FieldLevel) bool {
	return cert.IsCertificate(fl.Field().String())
}

func isPrivateKey(fl val.FieldLevel) bool {
	return cert.IsPrivateKey(fl.Field().String())
}

func isCertificatePath(fl val.FieldLevel) bool {
	return cert.IsCertificatePath(fl.Field().String())
}

func isPrivateKeyPath(fl val.FieldLevel) bool {
	return cert.IsPrivateKeyPath(fl.Field().String())
}
