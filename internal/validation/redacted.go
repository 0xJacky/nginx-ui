package validation

import (
	"github.com/0xJacky/Nginx-UI/settings"
	val "github.com/go-playground/validator/v10"
)

func redacted(fl val.FieldLevel) bool {
	return fl.Field().String() == settings.RedactedSensitiveValue
}
